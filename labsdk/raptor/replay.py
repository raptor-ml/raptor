# -*- coding: utf-8 -*-
# Copyright (c) 2022 RaptorML authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
import os.path
import types as pytypes
from datetime import datetime, timezone
from typing import Tuple, Optional, Union, Callable

import bentoml
import pandas as pd
from pandas.tseries.frequencies import to_offset

from . import local_state
from .program import Context, primitive, selector_regex, normalize_fqn
from .types.feature import FeatureSpec, Keys
from .types.model import ModelSpec
from .types.primitives import Primitive


def __detect_ts_field(df) -> Optional[str]:
    if 'timestamp' in df.columns:
        return 'timestamp'
    elif 'time' in df.columns:
        return 'time'
    elif 'date' in df.columns:
        return 'date'
    elif 'datetime' in df.columns:
        return 'datetime'
    elif 'ts' in df.columns:
        return 'ts'
    elif 'event_timestamp' in df.columns:
        return 'event_timestamp'
    elif 'event_at' in df.columns:
        return 'event_at'
    elif 'event_time' in df.columns:
        return 'event_time'
    elif 'event_date' in df.columns:
        return 'event_date'
    elif 'event_datetime' in df.columns:
        return 'event_datetime'
    elif 'event_ts' in df.columns:
        return 'event_ts'

    # Try to detect the timestamp field by data type
    for col in df.columns:
        try:
            if isinstance(df[col].iloc[0], (pd.Timestamp, pd.DatetimeIndex, datetime)):
                return col
            elif isinstance(df[col].iloc[0], str):
                try:
                    pd.to_datetime(df[col].iloc[0])
                    return col
                except (ValueError, TypeError):
                    pass
        except (KeyError, IndexError):
            pass
    return None


def __detect_headers_field(df) -> Optional[str]:
    if 'headers' in df.columns:
        return 'headers'
    else:
        return None


def new_replay(spec: FeatureSpec):
    def _replay(store_locally=True):
        df: Optional[pd.DataFrame] = None
        timestamp_field: Optional[str] = None

        if spec.data_source_spec is not None:
            dsrc = spec.data_source_spec
            df = dsrc.local_df.copy()
            timestamp_field = dsrc.timestamp
            if spec.keys is None:
                spec.keys = dsrc.keys
        elif spec.sourceless_df is not None:
            df = spec.sourceless_df.copy()

        if df is None:
            raise ValueError('Cannot replay feature spec without data source that was registered in the LabSDK.')

        if timestamp_field is None:
            timestamp_field = __detect_ts_field(df)
            if timestamp_field is None:
                raise Exception('No `timestamp` field detected for the dataframe.\n'
                                '   Please specify using the `timestamp_field` argument of the `DataSource`.')

        if spec.keys is None:
            raise Exception('No key fields defined for the dataframe.\n'
                            '   Please specify using the `keys` argument of the `Feature`.')

        # normalize
        df[timestamp_field] = pd.to_datetime(df[timestamp_field])
        for k in spec.keys:
            df[k] = df[k].astype(str)

        df['__raptor.ret__'] = df.apply(__replay_map(spec, timestamp_field), axis=1)
        df = df.dropna(subset=['__raptor.ret__'])
        if df.empty:
            raise Exception('No data returned from the feature spec.')
        df['__raptor.keys__'] = df.apply(lambda row: Keys({k: row[k] for k in spec.keys}).encode(spec), axis=1)

        # flip dataframe to feature_value df
        feature_values = df.filter(['__raptor.keys__', '__raptor.ret__', timestamp_field], axis=1).rename(columns={
            '__raptor.keys__': 'keys',
            '__raptor.ret__': 'value',
            timestamp_field: 'timestamp',
        })

        if spec.aggr is None:
            feature_values.insert(0, 'fqn', spec.fqn())
            if store_locally:
                local_state.store_feature_values(feature_values)
            return feature_values

        # aggregations
        feature_values = feature_values.set_index('timestamp').sort_index()
        win = to_offset(spec.staleness)
        fields = []

        val_field = 'value'
        if spec.primitive == Primitive.String:
            feature_values['f_value'] = feature_values['value'].factorize()[0]
            val_field = 'f_value'

        # TODO: refactor this to use bucketing
        fvg = feature_values.groupby(['keys']).rolling(win)[val_field]

        for aggr in spec.aggr.funcs:
            f = f'{spec.fqn()}+{aggr.value}'
            result = aggr.apply(fvg).reset_index(0).rename(columns={'value': f})
            feature_values = feature_values.merge(result, on=['timestamp', 'keys'], how='left')
            fields.append(f)

        if 'f_value' in feature_values.columns:
            feature_values = feature_values.drop('f_value', axis=1)

        feature_values = feature_values.reset_index().drop(columns=['value']). \
            melt(id_vars=['timestamp', 'keys'], value_vars=fields, var_name='fqn', value_name='value')
        if store_locally:
            local_state.store_feature_values(feature_values)
        return feature_values

    def replay(store_locally=True):
        """Replay a dataframe on the feature definition to create features values from existing data.

        :param bool store_locally: Store the data locally in the feature values (this is required for working with other
            capabilities such as using the feature as a dependency or adding the data to a FeatureSet). Default is True.
        :return: pd.DataFrame with the calculated feature values
        """

        try:
            return _replay(store_locally)

        except Exception as e:
            back_frame = e.__traceback__.tb_frame
            while back_frame.f_code.co_filename.startswith(os.path.dirname(__file__)):
                back_frame = back_frame.f_back
            tb = pytypes.TracebackType(tb_next=None,
                                       tb_frame=back_frame,
                                       tb_lasti=back_frame.f_lasti,
                                       tb_lineno=back_frame.f_lineno)
            raise Exception(f'{spec.program.name}: {str(e)}').with_traceback(tb)

    return replay


def _prediction_getter(owner_spec: FeatureSpec) -> Callable[[str, Keys, datetime], Tuple[primitive, datetime]]:
    def get(selector: str, keys: Keys, timestamp: datetime) -> Tuple[primitive, datetime]:
        spec = local_state.spec_by_selector(selector)
        if spec is None:
            raise Exception(f'Model not found: {selector}')
        if not isinstance(spec, ModelSpec):
            raise Exception(f'Spec found, but not a model: {selector}')

        model: bentoml.Model = spec.exporter.get_model()
        if model is None:
            raise Exception(f'Model not found: {selector}')

        ts = pd.to_datetime(timestamp)
        data = {}
        for fqn in spec.features:
            fg = _feature_getter(owner_spec)
            value, _ = fg(fqn, keys, ts)
            data[fqn] = value

        df = pd.DataFrame([data])
        return model.to_runner().run(df), ts

    return get


def _feature_getter(owner_spec: FeatureSpec) -> Callable[
    [str, Keys, datetime], Tuple[Optional[primitive], Optional[datetime]]]:
    def get(selector: str, keys: Keys, timestamp: datetime) -> Tuple[Optional[primitive], Optional[datetime]]:
        spec = local_state.spec_by_selector(selector)
        if spec is None:
            raise Exception(f'Feature not found: {selector}')
        if isinstance(spec, ModelSpec):
            return _prediction_getter(owner_spec)(selector, keys, timestamp)

        ts = pd.to_datetime(timestamp)

        # TODO: refactor this to support bucketing
        df = local_state.__feature_values

        matches = selector_regex.match(selector)
        if matches is None:
            raise Exception(f'Invalid selector: {selector}')

        fqn = normalize_fqn(selector, owner_spec.namespace)
        if matches.group('aggrFn') is not None:
            fqn = f'{fqn}+{matches.group("aggrFn")}'
            if matches.group('version') is not None:
                raise Exception(f'Cannot specify previous version for aggregated feature: {selector}')

        version = 0
        if matches.group('version') is not None:
            version = int(matches.group('version'))
            if version < 0:
                version *= -1

        if version > 0:
            if spec.keep_previous is None:
                raise Exception(
                    f'''selector specified a previous version, although the feature doesn't support it: {selector}'''
                )
            if version > spec.keep_previous.versions:
                raise Exception(
                    f'selector specified a previous version({version}) which is greater than the configured '
                    f'keep_previous ({spec.keep_previous.versions}): {selector}'
                )

        df = df.loc[(df['fqn'] == fqn) & (df['keys'] == keys.encode(spec)) & (df['timestamp'] <= ts)]

        if version > 0:
            df = df.sort_values(by=['timestamp'], ascending=False).head(version + 1)
            if df.empty:
                return None, None
            if len(df) < version:
                return None, None
            if len(df) < version + 1:
                return None, None
            res = df.iloc[version]

            if spec.keep_previous.over.total_seconds() > 0:
                ts_of_last = df.iloc[0]['timestamp']
                if res['timestamp'] < ts_of_last - (version * spec.keep_previous.over):
                    return None, None

            return res['value'], res['timestamp']

        if spec.staleness.total_seconds() > 0:
            df = df.loc[(df['timestamp'] >= ts - spec.staleness)]

        df = df.sort_values(by=['timestamp'], ascending=False).head(1)
        if df.empty:
            return None, None
        res = df.iloc[0]

        return res['value'], res['timestamp']

    return get


def __replay_map(spec: FeatureSpec, timestamp_field: str):
    def map(row: pd.Series):
        ts = row[timestamp_field]
        row = row.drop(timestamp_field)

        keys = Keys()
        for k in spec.keys:
            keys[k] = row[k]
            row = row.drop(k)

        data = {}
        for k, v in row.items():
            data[str(k)] = v
        return spec.program.call(
            data=data,
            context=Context(
                spec.fqn(),
                keys=keys,
                timestamp=ts,
                feature_getter=_feature_getter(spec),
                prediction_getter=_prediction_getter(spec),
            )
        )

    return map


def new_historical_get(spec: ModelSpec):
    if not isinstance(spec, ModelSpec):
        raise Exception('Not a Model')

    def _historical_get(since: Optional[Union[datetime, str]] = None, until: Optional[Union[datetime, str]] = None):
        if since == '' or since == '-1':
            since = None
        if until == '' or until == '-1':
            until = None
        if isinstance(since, str):
            since = pd.to_datetime(since)
        if isinstance(until, str):
            until = pd.to_datetime(until)

        if since is not None and until is not None and since > until:
            raise Exception('since > until')

        if since is not None and since.tzinfo is None:
            since = since.replace(tzinfo=timezone.utc)
        if until is not None and until.tzinfo is None:
            until = until.replace(tzinfo=timezone.utc)

        key_feature = spec.key_feature
        _key_feature_spec = local_state.feature_spec_by_selector(key_feature)
        features = spec.features + spec.label_features

        if key_feature in features:
            features.remove(key_feature)

        df = local_state.feature_values()
        if df.empty:
            raise Exception('No data found. Have you Replayed on your data?')

        df = df.loc[(df['fqn'].isin(features + [key_feature]))]

        if since is not None:
            df = df.loc[(df['timestamp'] >= since)]
        if until is not None:
            df = df.loc[(df['timestamp'] <= until)]

        if df.empty:
            raise Exception('No data found')

        key_df = df.loc[df['fqn'] == key_feature]

        key_df = key_df.rename(columns={'value': key_feature})
        key_df = key_df.drop(columns=['fqn'])

        for f in features:
            f_spec = local_state.feature_spec_by_selector(f)

            f_df = df.loc[df['fqn'] == f]
            f_df = f_df.rename(columns={'value': f})
            f_df = f_df.drop(columns=['fqn'])
            # f_df["start_ts"] = f_df["end_ts"] - f_staleness

            if f_spec.staleness.total_seconds() > 0:
                key_df = pd.merge_asof(key_df.sort_values('timestamp'), f_df.sort_values('timestamp'), on='timestamp',
                                       by='keys', direction='nearest', tolerance=f_spec.staleness)
            else:
                key_df = pd.merge_asof(key_df.sort_values('timestamp'), f_df.sort_values('timestamp'), on='timestamp',
                                       by='keys', direction='nearest')

        return key_df.reset_index(drop=True)

    def historical_get(since: Optional[Union[datetime, str]] = None, until: Optional[Union[datetime, str]] = None):
        """
        Get historical data for a FeatureSet.
        :param datetime|str since: start time of the query
        :param datetime|str until: end time of the query
        :return: pd.DataFrame with the historical data of the FeatureSet
        """

        try:
            return _historical_get(since, until)
        except Exception as e:
            back_frame = e.__traceback__.tb_frame
            while back_frame.f_code.co_filename.startswith(os.path.dirname(__file__)):
                back_frame = back_frame.f_back
            tb = pytypes.TracebackType(tb_next=None,
                                       tb_frame=back_frame,
                                       tb_lasti=back_frame.f_lasti,
                                       tb_lineno=back_frame.f_lineno)
            raise Exception(f'{spec.fqn()}: {str(e)}').with_traceback(tb)

    return historical_get

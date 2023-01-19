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

import types as pytypes
from datetime import datetime, timezone
from typing import Dict, Tuple, Optional, Union

import pandas as pd
from pandas.tseries.frequencies import to_offset

from . import local_state
from .program import Context, primitive
from .types import FeatureSpec, ModelSpec, Keys, Primitive


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
    else:
        return None


def __detect_headers_field(df) -> Optional[str]:
    if 'headers' in df.columns:
        return 'headers'
    else:
        return None


def new_replay(spec: FeatureSpec):
    def _replay(store_locally=True):
        dsrc = spec._data_source_spec
        if dsrc is None:
            raise ValueError('Cannot replay feature spec without data source that was registered in the LabSDK.')

        df = dsrc._local_df.copy()

        timestamp_field = dsrc.timestamp
        if timestamp_field is None:
            timestamp_field = __detect_ts_field(df)
            if timestamp_field is None:
                raise Exception('No `timestamp` field detected for the dataframe.\n'
                                '   Please specify using the `timestamp_field` argument of the `DataSource`.')

        if spec.keys is None:
            spec.keys = dsrc.keys

        if spec.keys is None:
            raise Exception('No key fields defined for the dataframe.\n'
                            '   Please specify using the `keys` argument of the `Feature`.')

        # normalize
        df[timestamp_field] = pd.to_datetime(df[timestamp_field])
        for k in spec.keys:
            df[k] = df[k].astype(str)

        df['__raptor.ret__'] = df.apply(__replay_map(spec, timestamp_field), axis=1)
        df = df.dropna(subset=['__raptor.ret__'])
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
        feature_values = feature_values.set_index('timestamp')
        win = to_offset(spec.staleness)
        fields = []

        val_field = 'value'
        if spec.primitive == Primitive.String:
            feature_values['f_value'] = feature_values['value'].factorize()[0]
            val_field = 'f_value'

        # TODO: refactor this to use bucketing
        for aggr in spec.aggr.funcs:
            f = f'{spec.fqn()}+{aggr.value}'
            feature_values[f] = aggr.apply(feature_values.groupby(['keys']).rolling(win)[val_field]). \
                reset_index(0, drop=True)
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
            back_frame = e.__traceback__.tb_frame.f_back
            tb = pytypes.TracebackType(tb_next=None,
                                       tb_frame=back_frame,
                                       tb_lasti=back_frame.f_lasti,
                                       tb_lineno=back_frame.f_lineno)
            raise Exception(f'{spec.program.name}: {str(e)}').with_traceback(tb)

    return replay


def _prediction_getter(selector: str, keys: Dict[str, str], timestamp: datetime) -> Tuple[primitive, datetime]:
    raise NotImplementedError('TBD')


def _feature_getter(selector: str, keys: Dict[str, str], timestamp: datetime) -> Tuple[Optional[primitive], Optional[datetime]]:
    spec = local_state.feature_spec_by_selector(selector)
    ts = pd.to_datetime(timestamp)

    # TODO: refactor this to support bucketing
    df = local_state.__feature_values
    df = df.loc[(df['fqn'] == selector) & (df['keys'] == keys) & (df['timestamp'] <= ts)]

    if spec.staleness.total_seconds() > 0:
        df = df.loc[(df['timestamp'] >= ts - spec.staleness)]

    df = df.sort_values(by=['timestamp'], ascending=False).head(1)
    if df.empty:
        return None, None
    res = df.iloc[0]

    return res['value'], res['timestamp']


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
                feature_getter=_feature_getter,
                prediction_getter=_prediction_getter,
            )
        )

    return map


def new_historical_get(spec):
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
        features = spec.features
        features += spec.label_features

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
            back_frame = e.__traceback__.tb_frame.f_back.f_back
            tb = pytypes.TracebackType(tb_next=None,
                                       tb_frame=back_frame,
                                       tb_lasti=back_frame.f_lasti,
                                       tb_lineno=back_frame.f_lineno)
            raise Exception(f'{spec.fqn()}: {str(e)}').with_traceback(tb)

    return historical_get

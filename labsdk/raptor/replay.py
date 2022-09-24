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

import datetime
import json
import types as pytypes

import pandas as pd
from pandas.tseries.frequencies import to_offset

from . import local_state, replay_instructions
from .pyexp import pyexp, go
from .types import FeatureSpec, Primitive, _wrap_exception, FeatureSetSpec, PyExpException


def __detect_ts_field(df) -> str:
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


def __detect_headers_field(df) -> str:
    if 'headers' in df.columns:
        return 'headers'
    else:
        return None


def __detect_entity_id(df) -> str:
    if 'entity_id' in df.columns:
        return 'entity_id'
    elif 'entityId' in df.columns:
        return 'entityId'
    elif 'entityID' in df.columns:
        return 'entityID'
    else:
        return None


def new_replay(spec: FeatureSpec):
    def _replay(df: pd.DataFrame, timestamp_field: str = None, headers_field: str = None, entity_id_field: str = None,
                store_locally=True):

        df = df.copy()

        if timestamp_field is None:
            timestamp_field = __detect_ts_field(df)
            if timestamp_field is None:
                raise Exception("No `timestamp` field detected for the dataframe.\n"
                                "   Please specify using the `timestamp_field` argument")

        if entity_id_field is None:
            entity_id_field = __detect_entity_id(df)
            if entity_id_field is None:
                raise Exception("No `entity_id` field detected for the dataframe.\n"
                                "   Please specify using the `entity_id_field` argument")

        if headers_field is None:
            headers_field = __detect_headers_field(df)

        # normalize
        df[timestamp_field] = pd.to_datetime(df[timestamp_field])
        df[entity_id_field] = df[entity_id_field].astype(str)

        df["__raptor.ret__"] = df.apply(__replay_map(spec, timestamp_field, headers_field, entity_id_field), axis=1)
        df = df.dropna(subset=['__raptor.ret__'])

        # flip dataframe to feature_value df
        feature_values = df.filter([entity_id_field, "__raptor.ret__", timestamp_field], axis=1).rename(columns={
            entity_id_field: "entity_id",
            "__raptor.ret__": "value",
            timestamp_field: "timestamp",
        })

        if spec.aggr is None:
            feature_values.insert(0, "fqn", spec.fqn())
            if store_locally:
                local_state.store_feature_values(feature_values)
                fv = local_state.feature_values()
                return fv.loc[fv["fqn"] == spec.freshness]
            return feature_values

        # aggregations
        feature_values = feature_values.set_index('timestamp')
        win = to_offset(spec.staleness)
        fields = []

        val_field = "value"
        if spec.primitive == Primitive.String:
            feature_values["f_value"] = feature_values["value"].factorize()[0]
            val_field = "f_value"

        for aggr in spec.aggr.funcs:
            f = f'{spec.fqn()}[{aggr.value}]'

            feature_values[f] = aggr.apply(feature_values.groupby(["entity_id"]).rolling(win)[val_field]). \
                reset_index(0, drop=True)

            fields.append(f)

        if "f_value" in feature_values.columns:
            feature_values = feature_values.drop("f_value", axis=1)

        feature_values = feature_values.reset_index().drop(columns=["value"]). \
            melt(id_vars=["timestamp", "entity_id"], value_vars=fields,
                 var_name="fqn", value_name="value")
        if store_locally:
            local_state.store_feature_values(feature_values)
            fv = local_state.feature_values()
            return fv.loc[fv["fqn"] == spec.fqn()]
        return feature_values

    def replay(df: pd.DataFrame, timestamp_field: str = None, headers_field: str = None, entity_id_field: str = None,
               store_locally=True):
        """Replay a dataframe on the feature definition to create features values from existing data.

        :param pd.DataFrame df: pandas dataframe with the data to replay
        :param Optional[str] timestamp_field: the name of the column containing the timestamp of the data.
        :param Optional[str] headers_field: the name of the column containing the headers of the data.
        :param Optional[str] entity_id_field: the name of the column containing the entity id of the data.
        :param bool store_locally: Store the data locally in the feature values (this is required for working with other
            capabilities such as using the feature as a dependency or adding the data to a FeatureSet). Default is True.
        :return: pd.DataFrame with the calculated feature values
        """

        try:
            return _replay(df, timestamp_field, headers_field, entity_id_field, store_locally)
        except PyExpException as e:
            raise e
        except Exception as e:
            back_frame = e.__traceback__.tb_frame.f_back
            tb = pytypes.TracebackType(tb_next=None,
                                       tb_frame=back_frame,
                                       tb_lasti=back_frame.f_lasti,
                                       tb_lineno=back_frame.f_lineno)
            raise Exception(f"{spec.program.name}: {str(e)}").with_traceback(tb)

    return replay


def __dependency_getter(fqn, eid, ts, val):
    try:
        spec = local_state.spec_by_fqn(fqn)
        ts = pd.to_datetime(ts)

        df = local_state.__feature_values
        df = df.loc[(df["fqn"] == fqn) & (df["entity_id"] == eid) & (df["timestamp"] <= ts)]

        if spec.staleness.total_seconds() > 0:
            df = df.loc[(df["timestamp"] >= ts - spec.staleness)]

        df = df.sort_values(by=["timestamp"], ascending=False).head(1)
        if df.empty:
            return str.encode("")
        res = df.iloc[0]

        v = pyexp.PyVal(handle=val)

        v.Value = json.dumps(res["value"])
        v.Timestamp = pyexp.PyTime(res["timestamp"].isoformat("T"), "")
        v.Fresh = True

        if spec.freshness.total_seconds() > 0:
            v.Fresh = res["timestamp"] >= ts - spec.freshness

    except Exception as e:
        """return error"""
        return str.encode(str(e))

    return str.encode("")


def __replay_map(spec: FeatureSpec, timestamp_field: str, headers_field: str = None,
                 entity_id_field: str = None):
    def map(row: pd.Series):
        ts = row[timestamp_field]
        row = row.drop(timestamp_field)

        if ts.tzinfo is None:
            ts = ts.tz_localize('UTC')

        headers = go.nil
        if headers_field is not None:
            headers = row[headers_field]
            # row = row.drop(headers_field)

        entity_id = ""
        if entity_id_field is not None:
            entity_id = row[entity_id_field]
            # row = row.drop(entity_id_field)

        if isinstance(ts, datetime.datetime):
            ts = ts.isoformat("T")

        req = pyexp.PyExecReq(row.to_json(), __dependency_getter)
        req.Timestamp = pyexp.PyTime(ts, "")
        req.EntityID = entity_id
        req.Headers = headers

        try:
            res = spec.program.runtime.Exec(req)
            for i in res.Instructions:
                inst = pyexp.Instruction(handle=i)
                replay_instructions.__exec_instruction(inst)
            return json.loads(pyexp.JsonAny(res, "Value"))
        except RuntimeError as e:
            raise _wrap_exception(e, spec.program)
        except Exception as err:
            raise err

    return map


def new_historical_get(spec):
    def _historical_get(since: datetime.datetime, until: datetime.datetime):
        if not isinstance(spec, FeatureSetSpec):
            raise Exception("Not a FeatureSet")
        if isinstance(since, str):
            since = pd.to_datetime(since)
        if isinstance(until, str):
            until = pd.to_datetime(until)

        if since > until:
            raise Exception("since > until")

        if since.tzinfo is None:
            since = since.replace(tzinfo=datetime.timezone.utc)
        if until.tzinfo is None:
            until = until.replace(tzinfo=datetime.timezone.utc)

        key_feature = spec.key_feature
        _key_feature_spec = local_state.spec_by_fqn(key_feature)
        features = spec.features

        if key_feature in features:
            features.remove(key_feature)

        df = local_state.feature_values()
        if df.empty:
            raise Exception("No data found. Have you Replayed on your data?")

        df = df.loc[(df["fqn"].isin(features + [key_feature]))
                    & (df["timestamp"] >= since)
                    & (df["timestamp"] <= until)
                    ]

        if df.empty:
            raise Exception("No data found")

        key_df = df.loc[df["fqn"] == key_feature]

        key_df = key_df.rename(columns={"value": key_feature})
        key_df = key_df.drop(columns=["fqn"])

        for f in features:
            f_spec = local_state.spec_by_fqn(f)

            f_df = df.loc[df["fqn"] == f]
            f_df = f_df.rename(columns={"value": f})
            f_df = f_df.drop(columns=["fqn"])
            # f_df["start_ts"] = f_df["end_ts"] - f_staleness

            if f_spec.staleness.total_seconds() > 0:
                key_df = pd.merge_asof(key_df.sort_values("timestamp"), f_df.sort_values("timestamp"), on="timestamp",
                                       by="entity_id", direction="nearest", tolerance=f_spec.staleness)
            else:
                key_df = pd.merge_asof(key_df.sort_values("timestamp"), f_df.sort_values("timestamp"), on="timestamp",
                                       by="entity_id", direction="nearest")

        return key_df.reset_index(drop=True)

    def historical_get(since: datetime.datetime, until: datetime.datetime):
        """
        Get historical data for a FeatureSet.
        :param datetime.datetime|str since: start time of the query
        :param datetime.datetime|str until: end time of the query
        :return: pd.DataFrame with the historical data of the FeatureSet
        """

        try:
            return _historical_get(since, until)
        except Exception as e:
            back_frame = e.__traceback__.tb_frame.f_back
            tb = pytypes.TracebackType(tb_next=None,
                                       tb_frame=back_frame,
                                       tb_lasti=back_frame.f_lasti,
                                       tb_lineno=back_frame.f_lineno)
            raise Exception(f"{spec.program.name}: {str(e)}").with_traceback(tb)

    return historical_get

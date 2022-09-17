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

import pandas
import pandas as pd

from . import local_state
from .types import Primitive
from .pyexp import pyexp


def _inst_spec(fqn):
    spec = local_state.spec_by_fqn(fqn)
    if spec is None:
        raise ValueError(f"Unknown FQN {fqn}")
    if spec.aggr is not None:
        raise Exception("Aggregation is not supported for Replay's effects at the moment")
    return spec


def __exec_instruction(inst: pyexp.Instruction):
    if inst.Operation == pyexp.InstructionOpSet:
        return _exec_set(inst)
    if inst.Operation == pyexp.InstructionOpUpdate:
        return _exec_update(inst)
    if inst.Operation == pyexp.InstructionOpAppend:
        return _exec_append(inst)
    if inst.Operation == pyexp.InstructionOpIncr:
        return _exec_incr(inst)


def _exec_append(inst):
    spec = _inst_spec(inst.FQN)
    if spec.primitive.is_scalar():
        raise Exception("Append is not supported for scalars")
    df = _get_recent(inst)

    ts = pd.to_datetime(pyexp.PyTimeRFC3339(inst.Timestamp))
    o = {
        "entity_id": inst.EntityID,
        "value": [pyexp.JsonAny(inst, "Value")],
        "timestamp": ts,
        "fqn": inst.FQN,
    }
    if df is not None:
        o["value"] = df["value"] + o["value"]

    local_state.store_feature_values(pandas.DataFrame.from_records([o]))


def _exec_incr(inst):
    spec = _inst_spec(inst.FQN)
    if spec.primitive not in [Primitive.Integer, Primitive.Float]:
        raise Exception("Incr is only supported for numbers")

    df = _get_recent(inst)
    ts = pd.to_datetime(pyexp.PyTimeRFC3339(inst.Timestamp))
    o = {
        "entity_id": inst.EntityID,
        "value": pyexp.JsonAny(inst, "Value"),
        "timestamp": ts,
        "fqn": inst.FQN,
    }
    if df is not None:
        if o["value"] is None:
            o["value"] = 0

        val = float(df["value"]) + float(o["value"])
        if spec.primitive == "int":
            val = int(val)
        o["value"] = val

    local_state.store_feature_values(pandas.DataFrame.from_records([o]))


def _get_recent(inst):
    spec = _inst_spec(inst.FQN)
    ts = pd.to_datetime(pyexp.PyTimeRFC3339(inst.Timestamp))

    df = local_state.__feature_values
    df = df.loc[(df["fqn"] == inst.FQN) & (df["entity_id"] == inst.EntityID) & (df["timestamp"] <= ts)]

    if spec.staleness.total_seconds() > 0:
        df = df.loc[(df["timestamp"] >= ts - spec.staleness)]

    df = df.sort_values(by=["timestamp"], ascending=False).head(1)
    if df.empty:
        return None
    return df.iloc[0]


def _exec_update(inst):
    spec = _inst_spec(inst.FQN)
    if not spec.primitive.is_scalar():
        return _exec_append(inst)
    pass


def _exec_set(inst: pyexp.Instruction):
    ts = pd.to_datetime(pyexp.PyTimeRFC3339(inst.Timestamp))
    local_state.store_feature_values(pandas.DataFrame.from_records([{
        "entity_id": inst.EntityID,
        "value": pyexp.JsonAny(inst, "Value"),
        "timestamp": ts,
        "fqn": inst.FQN,
    }]))

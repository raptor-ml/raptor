# Copyright (c) 2022 Raptor.
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

import re

import pandas as pd

from raptor import types

# registered features
spec_registry = []


def register_spec(spec):
    global spec_registry
    for idx, s in enumerate(spec_registry):
        if s["fqn"] == spec["fqn"]:
            spec_registry[idx] = spec
            return
    spec_registry.append(spec)


def check_valid_fqn(spec, fqn):
    if spec["kind"] != "feature":
        raise Exception(f"`{fqn}` is not a feature")
    if spec["fqn"] != fqn:
        fn = re.match(r"^(.*)\[(.*)\]$", fqn)
        if fn is not None:
            fn = types.AggrFn(fn.group(2))
            if "aggr" not in spec["options"]:
                err = f"feature `{fqn}` is not an aggregation"
                raise Exception(err)
            if fn not in spec["options"]["aggr"]:
                err = f"feature `{spec['fqn']}` doesn't include aggregation `{fn}`"
                raise Exception(err)
        else:
            raise Exception(f"feature `{fqn}` is not a invalid")


def spec_by_fqn(fqn: str):
    global spec_registry
    spec = next(filter(lambda m: m["kind"] == "feature" and m["fqn"] == fqn.split("[")[0], spec_registry), None)
    check_valid_fqn(spec, fqn)
    return spec


def spec_by_src_name(src_name: str):
    global spec_registry
    return next(filter(lambda m: m["kind"] == "feature" and m["src_name"] == src_name, spec_registry), None)


# Calculated feature values
__feature_values = pd.DataFrame()


def store_feature_values(feature_values):
    global __feature_values
    __feature_values = pd.concat([__feature_values, feature_values])


def feature_values():
    global __feature_values
    return __feature_values.copy()

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

from __future__ import annotations

import re

import pandas as pd

from .types import FeatureSpec, AggrFn, FeatureSetSpec, normalize_fqn

# registered features
spec_registry: [FeatureSpec | FeatureSetSpec] = []


def register_spec(spec):
    global spec_registry
    for idx, s in enumerate(spec_registry):
        if s.fqn() == spec.fqn():
            spec_registry[idx] = spec
            return
    spec_registry.append(spec)


def check_valid_fqn(spec, fqn):
    if not isinstance(spec, FeatureSpec):
        raise Exception(f"`{fqn}` is not a feature")
    if spec.fqn() != fqn:
        fn = re.match(r"^(.*)\[(.*)\]$", fqn)
        if fn is not None:
            fn = AggrFn(fn.group(2))
            if spec.aggr is None:
                err = f"feature `{fqn}` is not an aggregation"
                raise Exception(err)
            if fn not in spec.aggr.funcs:
                err = f"feature `{spec.fqn()}` doesn't include aggregation `{fn}`"
                raise Exception(err)
        else:
            raise Exception(f"feature `{fqn}` is not a invalid")


def spec_by_fqn(fqn: str) -> FeatureSpec:
    global spec_registry
    fqn = normalize_fqn(fqn)

    spec = next(filter(lambda m: isinstance(m, FeatureSpec) and m.fqn() == fqn.split("[")[0], spec_registry), None)
    if spec is None:
        raise Exception(f"feature `{fqn}` is not registered locally")
    check_valid_fqn(spec, fqn)
    return spec


def spec_by_src_name(src_name: str) -> FeatureSpec:
    global spec_registry
    return next(filter(lambda m: isinstance(m, FeatureSpec) and m.program.name == src_name, spec_registry), None)


# Calculated feature values
__feature_values = pd.DataFrame()


def store_feature_values(feature_values):
    global __feature_values
    __feature_values = pd.concat([__feature_values, feature_values])


def feature_values():
    global __feature_values
    return __feature_values.copy()


def manifests(save_to_tmp=False, print_manifests=False):
    """
    manifests will create a list of registered Raptor manifests ready to install for your kubernetes cluster

    If save_to_tmp is True, it will save the manifests to a temporary file and return the path to the file.
    Otherwise, it will print the manifests.

    :type save_to_tmp: if True, save the manifests to a temporary file and return the path to the file
    :type print_manifests: if True, print the manifests
    """
    global spec_registry

    mfts = []
    for spec in spec_registry:
        mfts.append(spec.manifest())

    if len(mfts) == 0:
        return ""

    ret = '---\n'.join(mfts)
    if save_to_tmp:
        import tempfile
        f = tempfile.NamedTemporaryFile(mode='w+t', delete=False)
        f.write(ret)
        file_name = f.name
        f.close()
        return file_name
    elif print_manifests:
        print(ret)
    else:
        return ret

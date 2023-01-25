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

from __future__ import annotations

from typing import List, Union

import pandas as pd

from . import config
from .program import selector_regex, normalize_fqn
from .types.dsrc import DataSourceSpec
from .types.feature import FeatureSpec, AggregationFunction
from .types.model import ModelSpec

# registered features
Spec = Union[FeatureSpec, ModelSpec, DataSourceSpec]
spec_registry: List[Spec] = []


def register_spec(spec):
    global spec_registry
    for idx, s in enumerate(spec_registry):
        if s.fqn() == spec.fqn():
            spec_registry[idx] = spec
            return
    spec_registry.append(spec)


def check_valid_feature_selector(spec, selector):
    if not isinstance(spec, FeatureSpec):
        raise Exception(f'`{selector}` is not a feature')
    if spec.fqn() != selector:
        parsed = selector_regex.match(selector)
        if parsed.group('aggrFn') is not None:
            fn = AggregationFunction(parsed.group('aggrFn'))
            if spec.aggr is None:
                err = f'feature `{selector}` is not an aggregated'
                raise Exception(err)
            if fn not in spec.aggr.funcs:
                err = f"feature `{spec.fqn()}` doesn't include aggregation `{fn}`"
                raise Exception(err)
        elif parsed.group('version') is not None:
            version = int(parsed.group('version'))
            if spec.keep_previous is None:
                err = f'''feature `{selector}` doesn't supports previous versions'''
                raise Exception(err)
            if version > spec.keep_previous.versions:
                err = f'''feature `{selector}` doesn't have version `{version}`'''
                raise Exception(err)
        else:
            raise Exception(f'Invalid selector. Got: {selector}')


def spec_by_selector(selector: str) -> Spec:
    global spec_registry
    fqn = normalize_fqn(selector, config.default_namespace)
    spec = next(filter(lambda m: m.fqn() == fqn, spec_registry), None)
    if spec is None:
        raise Exception(f'Spec for `{selector}` is not registered locally')
    return spec


def feature_spec_by_selector(selector: str) -> FeatureSpec:
    global spec_registry
    fqn = normalize_fqn(selector, config.default_namespace)
    spec = next(filter(lambda m: isinstance(m, FeatureSpec) and m.fqn() == fqn, spec_registry), None)
    if spec is None:
        raise Exception(f'feature `{selector}` is not registered locally')
    check_valid_feature_selector(spec, selector)
    return spec


def feature_spec_by_src_name(src_name: str) -> FeatureSpec:
    global spec_registry
    return next(filter(lambda m: isinstance(m, FeatureSpec) and m.program.name == src_name, spec_registry), None)


# Calculated feature values
__feature_values = pd.DataFrame()


def store_feature_values(values):
    global __feature_values
    __feature_values = pd.concat([__feature_values, values])


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
        return ''

    ret = '---\r\n'.join(mfts)
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

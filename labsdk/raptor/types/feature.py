# -*- coding: utf-8 -*-
#  Copyright (c) 2022 RaptorML authors.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

from datetime import timedelta
from typing import Optional, List, Dict
from warnings import warn

import pandas as pd
import yaml
from pandas.core.window import RollingGroupby
from typing_extensions import TypedDict

from .common import RaptorSpec, ResourceReference, _k8s_name, EnumSpec, RuntimeSpec
from .dsrc import DataSourceSpec
from .primitives import Primitive
from .. import durpy, local_state
from .._internal.exporter.general import GeneralExporter
from ..program import Program


class AggregationFunction(EnumSpec):
    Unknown = 'unknown'
    Sum = 'sum'
    Avg = 'avg'
    Max = 'max'
    Min = 'min'
    Count = 'count'
    DistinctCount = 'distinct_count'
    ApproxDistinctCount = 'approx_distinct_count'

    @staticmethod
    def parse(a):
        if isinstance(a, AggregationFunction):
            return a
        if isinstance(a, str):
            return AggregationFunction[a]
        raise Exception(f'Unknown AggregationFunction {a}')

    def supports(self, typ):
        if self == AggregationFunction.Unknown:
            return False
        if self in (AggregationFunction.Sum, AggregationFunction.Avg, AggregationFunction.Max, AggregationFunction.Min):
            return typ in (Primitive.Integer, Primitive.Float)
        return True

    def apply(self, rgb: RollingGroupby):
        if self == AggregationFunction.Sum:
            return rgb.sum()
        if self == AggregationFunction.Avg:
            return rgb.mean()
        if self == AggregationFunction.Max:
            return rgb.max()
        if self == AggregationFunction.Min:
            return rgb.min()
        if self == AggregationFunction.Count:
            return rgb.count()
        if self == AggregationFunction.DistinctCount or self == AggregationFunction.ApproxDistinctCount:
            return rgb.apply(lambda x: pd.Series(x).nunique())
        raise Exception(f'Unknown AggrFn {self}')


class BuilderSpec(RuntimeSpec):
    kind: str = None
    code: str = None

    def __init__(self, runtime: Optional[str] = None, packages: Optional[List[str]] = None, kind: Optional[str] = None,
                 options=None):
        super().__init__(runtime=runtime, packages=packages)
        self.kind = kind
        if options is not None:
            """add options to __dict__"""
            for k, v in options.items():
                setattr(self, k, v)


class AggrSpec(yaml.YAMLObject):
    funcs: [AggregationFunction] = None
    over: timedelta = None
    granularity: timedelta = None

    def __init__(self, fns: List[AggregationFunction], over: timedelta, granularity: timedelta):
        self.funcs = fns
        self.over = over
        self.granularity = granularity

    def __setattr__(self, key, value):
        if key == 'granularity':
            if value == '' or value is None:
                value = None
            elif isinstance(value, str):
                value = durpy.from_str(value)
            elif isinstance(value, timedelta):
                value = value
            else:
                raise Exception(f'Invalid type {type(value)} for {key}')

        super().__setattr__(key, value)


class KeepPreviousSpec(yaml.YAMLObject):
    versions: int = None
    over: timedelta = None

    def __init__(self, versions: int, over: timedelta):
        if versions < 0:
            versions *= -1
        if versions == 0:
            raise Exception('versions must be greater than 0, or do not specify keep_previous')
        self.versions = versions
        self.over = over


class FeatureSpec(RaptorSpec):
    primitive: Primitive = None
    _freshness: Optional[timedelta] = None
    staleness: timedelta = None
    timeout: timedelta = None
    keep_previous: Optional[KeepPreviousSpec] = None
    keys: [str] = None

    data_source: Optional[ResourceReference] = None
    data_source_spec: Optional[DataSourceSpec] = None
    sourceless_df: Optional[pd.DataFrame] = None
    builder: BuilderSpec = BuilderSpec()
    aggr: AggrSpec = None

    program: Program = None

    def __init__(self, keys=None, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.keys = keys or []

    def export(self, with_dependent_source=True):
        GeneralExporter.add_feature(self, with_dependent_source=with_dependent_source)
        GeneralExporter.export()

    @property
    def freshness(self):
        if self.aggr is not None and self.aggr.granularity is not None:
            return self.aggr.granularity

        return self._freshness

    @freshness.setter
    def freshness(self, value):
        if value is None or value == '':
            self._freshness = None
        elif isinstance(value, str):
            self._freshness = durpy.from_str(value)
        elif isinstance(value, timedelta):
            self._freshness = value

    def __setattr__(self, key, value):
        if key == 'primitive':
            value = Primitive.parse(value)
        elif key == 'data_source':
            if value is None:
                pass
            elif isinstance(value, ResourceReference):
                self.data_source_spec = local_state.spec_by_selector(value.fqn())
            elif isinstance(value, str):
                value = ResourceReference(value)
                self.data_source_spec = local_state.spec_by_selector(value.fqn())
            elif type(value) == type(TypedDict) or isinstance(value, DataSourceSpec):
                if type(value) == type(TypedDict):
                    if hasattr(value, 'raptor_spec'):
                        value = value.raptor_spec
                    else:
                        raise Exception(f'TypedDict {value} does not have raptor_spec')
                value.features.append(self)
                self.data_source_spec = value
                value = ResourceReference(value.name, value.namespace)
            else:
                raise Exception(f'Invalid type {type(value)} for {key}')

            if self.data_source_spec is None and value is not None:
                warn(
                    f'DataSource {value.fqn()} not registered locally on the LabSDK. '
                    f'This will prevent you from replaying this feature locally'
                )
        elif key == 'program':
            if isinstance(value, Program):
                pass
            elif callable(value):
                raise Exception('Function must be parsed first')
            else:
                raise Exception('program must be a Program')
        elif key == 'staleness' or key == 'timeout':
            if value == '' or value is None:
                value = None
            elif isinstance(value, str):
                value = durpy.from_str(value)
            elif isinstance(value, timedelta):
                value = value
            else:
                raise Exception(f'Invalid type {type(value)} for {key}')

        super().__setattr__(key, value)

    @classmethod
    def to_yaml_dict(cls, data: 'FeatureSpec'):
        if data.aggr is not None:
            data.builder.aggr = data.aggr.funcs
            data.builder.aggrGranularity = data.aggr.granularity
        data.builder.code = data.program.code

        data.annotations['a8r.io/description'] = data.description

        return {
            'apiVersion': 'k8s.raptor.ml/v1alpha1',
            'kind': 'Feature',
            'metadata': {
                'name': _k8s_name(data.name),
                'namespace': data.namespace,
                'labels': data.labels,
                'annotations': data.annotations
            },
            'spec': {
                'primitive': data.primitive.value,
                'freshness': data.freshness,
                'staleness': data.staleness,
                'timeout': data.timeout,
                'keepPrevious': data.keep_previous,
                'keys': data.keys,
                'dataSource': None if data.data_source is None else data.data_source.__dict__,
                'builder': data.builder,
            }
        }


class Keys(Dict[str, str]):
    def encode(self, spec: FeatureSpec) -> str:
        ret: List[str] = []
        for key in spec.keys:
            val = self.get(key)
            if val is None:
                raise Exception(f'missing key {key}')
            ret.append(val)
        return ';'.join(ret)

    def decode(self, spec: FeatureSpec, encoded_keys: str) -> 'Keys':
        parts = encoded_keys.split(';')
        if len(parts) != len(spec.keys):
            raise Exception(f'invalid key {encoded_keys}')
        for i, encoded_keys in enumerate(spec.keys):
            self[encoded_keys] = parts[i]
        return self

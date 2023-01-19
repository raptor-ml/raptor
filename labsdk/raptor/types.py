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

import re
from datetime import timedelta, datetime
from enum import Enum
from typing import Optional, List, Dict, Any
from warnings import warn

import pandas as pd
import yaml
from pandas.core.window import RollingGroupby
from typing_extensions import TypedDict

from . import durpy, local_state
from .config import default_namespace
from .model import ModelFramework, ModelServer
from .program import Program
from .yaml import RaptorDumper


def validate_timedelta(td: timedelta):
    if td.days > 0:
        raise ValueError(
            f'calendarial durations is not supported. please specify the duration in hours instead.'
            f'e.g. {td.days} days -> {td.days * 24}h')


def _k8s_name(name):
    name = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    name = re.sub('__([A-Z])', r'_\1', name)
    name = re.sub('([a-z0-9])([A-Z])', r'\1_\2', name)
    return name.replace('_', '-').lower()


class AggregationFunction(Enum):
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

    @classmethod
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'AggregationFunction'):
        return dumper.represent_scalar('!AggrFn', data.value)


RaptorDumper.add_representer(AggregationFunction, AggregationFunction.to_yaml)


class Primitive(Enum):
    String = 'string'
    Integer = 'int'
    Float = 'float'
    Boolean = 'bool'
    Timestamp = 'timestamp'
    StringList = '[]string'
    IntList = '[]int'
    FloatList = '[]float'
    BooleanList = '[]bool'
    TimestampList = '[]timestamp'

    def is_scalar(self):
        return self in (Primitive.String, Primitive.Integer, Primitive.Float, Primitive.Timestamp)

    @staticmethod
    def parse(p):
        if isinstance(p, Primitive):
            return p
        elif p == 'str' or p == str:
            return Primitive.String
        elif p == 'int' or p == int:
            return Primitive.Integer
        elif p == 'float' or p == float:
            return Primitive.Float
        elif p == 'bool' or p == bool:
            return Primitive.Boolean
        elif p == 'timestamp' or p == datetime:
            return Primitive.Timestamp
        elif p == '[]string' or p == List[str]:
            return Primitive.StringList
        elif p == '[]int' or p == List[int]:
            return Primitive.IntList
        elif p == '[]float' or p == List[float]:
            return Primitive.FloatList
        elif p == '[]bool' or p == List[bool]:
            return Primitive.BooleanList
        elif p == '[]timestamp' or p == List[datetime]:
            return Primitive.TimestampList
        else:
            raise Exception('Primitive type {p} not supported')


class BuilderSpec(object):
    kind: str = None
    code: str = None
    runtime: Optional[str] = None
    packages: Optional[List[str]] = None

    def __init__(self, kind: Optional[str], options=None):
        self.kind = kind
        if options is not None:
            """add options to __dict__"""
            for k, v in options.items():
                setattr(self, k, v)


class ResourceReference:
    name: str = None
    namespace: str = None

    def __init__(self, name, namespace=None):
        if namespace is None:
            parts = name.split('.')
            if len(parts) == 1:
                self.name = name
            else:
                self.namespace = parts[0]
                self.name = parts[1]
        self.name = _k8s_name(name)
        self.namespace = namespace

    def fqn(self):
        if self.namespace is None:
            return self.name
        return f'{self.namespace}.{self.name}'


class AggrSpec:
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


class RaptorSpec(yaml.YAMLObject):
    name: str = None
    namespace: str = None

    @classmethod
    def yaml_tag(cls):
        return f'!{cls.__class__.__name__}'

    def fqn(self):
        if self.namespace is None:
            return f'{default_namespace}.{self.name}'
        return f'{self.namespace}.{self.name}'

    def __str__(self):
        return self.__repr__()

    def __repr__(self):
        return f'{self.__class__.name}({self.fqn()})'

    def manifest(self, to_file: bool = False):
        """
        Returns the YAML manifest of the RaptorSpec
        :param to_file: if True, writes the manifest to a file in the
        :type to_file: bool
        :return: the YAML manifest
        """
        ret = yaml.dump(self, sort_keys=False, Dumper=RaptorDumper)
        ret = "# Generated by Raptor's LabSDK\r\n" + ret
        if to_file:
            typ = self.__class__.__name__.lower().replace('spec', '')
            with open(f'out/{typ}.{self.fqn().lower()}.yaml', 'w') as f:
                f.write(ret)
        return ret


class FeatureSpec(RaptorSpec):
    description: str = None
    labels: dict = {}
    annotations: dict = {}

    primitive: Primitive = None
    _freshness: Optional[timedelta] = None
    staleness: timedelta = None
    timeout: timedelta = None
    keys: [str] = None

    data_source: Optional[ResourceReference] = None
    _data_source_spec: Optional['DataSourceSpec'] = None
    builder: BuilderSpec = BuilderSpec(None)
    aggr: AggrSpec = None

    program: Program = None

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
                self._data_source_spec = local_state.spec_by_selector(value.fqn())
            elif isinstance(value, str):
                value = ResourceReference(value)
                self._data_source_spec = local_state.spec_by_selector(value.fqn())
            elif type(value) == type(TypedDict) or isinstance(value, DataSourceSpec):
                if type(value) == type(TypedDict):
                    if hasattr(value, 'raptor_spec'):
                        value = value.raptor_spec
                    else:
                        raise Exception(f'TypedDict {value} does not have raptor_spec')
                value.features.append(self)
                self._data_source_spec = value
                value = ResourceReference(value.name, value.namespace)
            else:
                raise Exception(f'Invalid type {type(value)} for {key}')

            if self._data_source_spec is None and value is not None:
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
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'FeatureSpec'):
        if data.aggr is not None:
            data.builder.aggr = data.aggr.funcs
            data.builder.aggrGranularity = data.aggr.granularity
        data.builder.code = data.program.code

        data.annotations['a8r.io/description'] = data.description

        manifest = {
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
                'keys': data.keys,
                'dataSource': None if data.data_source is None else data.data_source.__dict__,
                'builder': data.builder.__dict__,
            }
        }
        return dumper.represent_mapping(cls.yaml_tag(), manifest, flow_style=cls.yaml_flow_style)


RaptorDumper.add_representer(FeatureSpec, FeatureSpec.to_yaml)


class SecretKeyRef:
    name: str = None
    key: str = None

    def __init__(self, name, key):
        self.name = name
        self.key = key


class ConfigVar:
    name: str = None
    value: Optional[str] = None
    secretKeyRef: Optional[SecretKeyRef] = None

    def __init__(self, name, value=None, secret_key_ref=None):
        self.name = name
        self.value = value
        self.secretKeyRef = secret_key_ref
        if value is None and secret_key_ref is None:
            raise Exception('Must specify either value or secretKeyRef')


class ResourceRequirements:
    limits: Dict[str, str] = {}
    requests: Dict[str, str] = {}

    def __init__(self, limits=None, requests=None):
        self.limits = limits or {}
        self.requests = requests or {}


class DataSourceSpec(RaptorSpec):
    description: str = None
    labels: dict = {}
    annotations: dict = {}

    kind: str = None
    config: List[ConfigVar] = None
    schema: Optional[Dict[str, Any]] = None
    keys: List[str] = None
    timestamp: str = None
    replicas: int = None
    resources: ResourceRequirements = None

    features: List[FeatureSpec] = []

    _local_df: pd.DataFrame = None

    @classmethod
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'DataSourceSpec'):
        data.annotations['a8r.io/description'] = data.description
        manifest = {
            'apiVersion': 'k8s.raptor.ml/v1alpha1',
            'kind': 'DataSource',
            'metadata': {
                'name': _k8s_name(data.name),
                'namespace': data.namespace,
                'labels': data.labels,
                'annotations': data.annotations
            },
            'spec': {
                'kind': data.kind,
                'config': data.config,
                'keyFields': data.keys,
                'timestampField': data.timestamp,
                'replicas': data.replicas,
                'resources': data.resources,
                'schema': data.schema
            }
        }
        return dumper.represent_mapping(cls.yaml_tag(), manifest, flow_style=cls.yaml_flow_style)


RaptorDumper.add_representer(DataSourceSpec, DataSourceSpec.to_yaml)


class ModelSpec(RaptorSpec):
    description: str = None
    labels: dict = {}
    annotations: dict = {}

    keys: [str] = None

    freshness: Optional[timedelta] = None
    staleness: timedelta = None
    timeout: timedelta = None
    features: [str] = None
    label_features: [str] = None
    _key_feature: str = None

    model_framework: ModelFramework = None
    _model_framework_version: Optional[str] = None
    model_server: Optional[ModelServer] = None
    _trained_model: Optional[object] = None
    _training_code: Optional[str] = None

    _model_filename: Optional[str] = None

    def __setattr__(self, key, value):
        if key == 'freshness' or key == 'staleness' or key == 'timeout':
            if value == '' or value is None:
                value = None
            elif isinstance(value, str):
                value = durpy.from_str(value)
            elif isinstance(value, timedelta):
                value = value
            else:
                raise Exception(f'Invalid type {type(value)} for {key}')

        super().__setattr__(key, value)

    @property
    def key_feature(self):
        if self._key_feature is None:
            return self.label_features[0]
        return self._key_feature

    @key_feature.setter
    def key_feature(self, value):
        self._key_feature = value

    @classmethod
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'ModelSpec'):
        inference_config_stub = []
        for k, v in data.model_server.config.items():
            if isinstance(v, SecretKeyRef):
                inference_config_stub.append({'name': k, 'secretKeyRef': {'name': v.name, 'key': v.key}})
            else:
                inference_config_stub.append({'name': k, 'value': v})

        data.annotations['a8r.io/description'] = data.description
        manifest = {
            'apiVersion': 'k8s.raptor.ml/v1alpha1',
            'kind': 'Model',
            'metadata': {
                'name': _k8s_name(data.name),
                'namespace': data.namespace,
                'labels': data.labels,
                'annotations': data.annotations
            },
            'spec': {
                'freshness': data.freshness,
                'staleness': data.staleness,
                'timeout': data.timeout,
                'features': data.features,
                'keyFeature': None if data.key_feature == data.features[0] else data.key_feature,
                'labels': data.label_features,
                'modelFramework': data.model_framework,
                'modelFrameworkVersion': data._model_framework_version,
                'modelServer': data.model_server,
                'inferenceConfig': inference_config_stub,
                'storageURI': None if data._model_filename is None else f'$MODEL_BASE_URI/{data._model_filename}',
                'trainingCode': None if data._training_code is None else data._training_code,
            }
        }
        return dumper.represent_mapping(cls.yaml_tag(), manifest, flow_style=cls.yaml_flow_style)


RaptorDumper.add_representer(ModelSpec, ModelSpec.to_yaml)


class Keys(Dict[str, str]):
    def encode(self, spec: FeatureSpec) -> str:
        ret: List[str] = []
        for key in spec.keys:
            val = self.get(key)
            if val is None:
                raise Exception(f'missing key {key}')
            ret.append(val)
        return '.'.join(ret)

    def decode(self, spec: FeatureSpec, encoded_keys: str) -> 'Keys':
        parts = encoded_keys.split('.')
        if len(parts) != len(spec.keys):
            raise Exception(f'invalid key {encoded_keys}')
        for i, encoded_keys in enumerate(spec.keys):
            self[encoded_keys] = parts[i]
        return self

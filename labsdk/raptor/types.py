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
import re
from enum import Enum
from typing import Optional, List, Dict

import pandas as pd
import yaml
from pandas.core.window import RollingGroupby

from . import durpy
from .config import default_namespace
from .program import Program
from .yaml import RaptorDumper


def _k8s_name(name):
    name = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    name = re.sub('__([A-Z])', r'_\1', name)
    name = re.sub('([a-z0-9])([A-Z])', r'\1_\2', name)
    return name.replace("_", "-").lower()


class AggrFn(Enum):
    Unknown = 'unknown'
    Sum = 'sum'
    Avg = 'avg'
    Max = 'max'
    Min = 'min'
    Count = 'count'
    DistinctCount = 'distinct_count'
    ApproxDistinctCount = 'approx_distinct_count'

    def supports(self, typ):
        if self == AggrFn.Unknown:
            return False
        if self in (AggrFn.Sum, AggrFn.Avg, AggrFn.Max, AggrFn.Min):
            return typ in (Primitive.Integer, Primitive.Float)
        return True

    def apply(self, rgb: RollingGroupby):
        if self == AggrFn.Sum:
            return rgb.sum()
        if self == AggrFn.Avg:
            return rgb.mean()
        if self == AggrFn.Max:
            return rgb.max()
        if self == AggrFn.Min:
            return rgb.min()
        if self == AggrFn.Count:
            return rgb.count()
        if self == AggrFn.DistinctCount or self == AggrFn.ApproxDistinctCount:
            return rgb.apply(lambda x: pd.Series(x).nunique())
        raise Exception(f"Unknown AggrFn {self}")

    @classmethod
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'AggrFn'):
        return dumper.represent_scalar('!AggrFn', data.value)


RaptorDumper.add_representer(AggrFn, AggrFn.to_yaml)


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
        if p == 'str' or p == str:
            return Primitive.String
        elif p == 'int' or p == int:
            return Primitive.Integer
        elif p == 'float' or p == float:
            return Primitive.Float
        elif p == 'bool' or p == bool:
            return Primitive.Boolean
        elif p == 'timestamp' or p == datetime.datetime:
            return Primitive.Timestamp
        elif p == '[]string' or p == List[str]:
            return Primitive.StringList
        elif p == '[]int' or p == List[int]:
            return Primitive.IntList
        elif p == '[]float' or p == List[float]:
            return Primitive.FloatList
        elif p == '[]bool' or p == List[bool]:
            return Primitive.BooleanList
        elif p == '[]timestamp' or p == List[datetime.datetime]:
            return Primitive.TimestampList
        else:
            raise Exception("Primitive type not supported")


class BuilderSpec(object):
    kind: str = None
    pyexp: str = None

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
        self.name = name
        self.namespace = namespace


class AggrSpec:
    funcs: [AggrFn] = None
    granularity: datetime.timedelta = None

    def __init__(self, fns: [AggrFn], granularity: datetime.timedelta):
        self.funcs = fns
        self.granularity = granularity

    def __setattr__(self, key, value):
        if key == 'granularity':
            if value == '' or value is None:
                value = None
            elif isinstance(value, str):
                value = durpy.from_str(value)
            elif isinstance(value, datetime.timedelta):
                value = value
            else:
                raise Exception(f"Invalid type {type(value)} for {key}")

        super().__setattr__(key, value)


class FeatureSpec(yaml.YAMLObject):
    yaml_tag = u'!FeatureSpec'

    name: str = None
    namespace: str = None
    description: str = None
    labels: dict = {}
    annotations: dict = {}

    primitive: Primitive = None
    _freshness: Optional[datetime.timedelta] = None
    staleness: datetime.timedelta = None
    timeout: datetime.timedelta = None
    keys: [str] = None

    data_source: ResourceReference = None
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
        elif isinstance(value, datetime.timedelta):
            self._freshness = value

    def __setattr__(self, key, value):
        if key == 'primitive':
            value = Primitive.parse(value)
        elif key == 'program':
            if isinstance(value, Program):
                pass
            elif callable(value):
                raise Exception("Function must be parsed first")
            else:
                raise Exception("program must be a callable or a PyExpProgram")
        elif key == 'staleness' or key == 'timeout':
            if value == '' or value is None:
                value = None
            elif isinstance(value, str):
                value = durpy.from_str(value)
            elif isinstance(value, datetime.timedelta):
                value = value
            else:
                raise Exception(f"Invalid type {type(value)} for {key}")

        super().__setattr__(key, value)

    def fqn(self):
        if self.namespace is None:
            return f"{default_namespace}.{self.name}"
        return f"{self.namespace}.{self.name}"

    def __str__(self):
        return self.__repr__()

    def __repr__(self):
        return f"FeatureSpec({self.fqn()})"

    def manifest(self):
        """return a Kubernetes YAML manifest for this Feature definition"""
        return yaml.dump(self, sort_keys=False, Dumper=RaptorDumper)

    @classmethod
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'FeatureSpec'):
        if data.aggr is not None:
            data.builder.aggr = data.aggr.funcs
            data.builder.aggrGranularity = data.aggr.granularity
        data.builder.code = data.program.code

        data.annotations['a8r.io/description'] = data.description

        manifest = {
            "apiVersion": "k8s.raptor.ml/v1alpha1",
            "kind": "Feature",
            "metadata": {
                "name": _k8s_name(data.name),
                "namespace": data.namespace,
                "labels": data.labels,
                "annotations": data.annotations
            },
            "spec": {
                "primitive": data.primitive.value,
                "freshness": data.freshness,
                "staleness": data.staleness,
                "timeout": data.timeout,
                "keys": data.keys,
                "dataSource": None if data.data_source is None else data.data_source.__dict__,
                "builder": data.builder.__dict__,
            }
        }
        return dumper.represent_mapping(cls.yaml_tag, manifest, flow_style=cls.yaml_flow_style)


RaptorDumper.add_representer(FeatureSpec, FeatureSpec.to_yaml)


class FeatureSetSpec(yaml.YAMLObject):
    yaml_tag = u'!FeatureSetSpec'

    name: str = None
    namespace: str = None
    description: str = None
    labels: dict = {}
    annotations: dict = {}

    timeout: datetime.timedelta = None
    features: [str] = None
    _key_feature: str = None

    def fqn(self):
        if self.namespace is None:
            return f"{self.namespace}.{self.name}"
        return f"{self.namespace}.{self.name}"

    def __str__(self):
        return self.__repr__()

    def __repr__(self):
        return f"FeatureSetSpec({self.fqn()})"

    def __setattr__(self, key, value):
        if key == 'timeout':
            if isinstance(value, str):
                value = durpy.from_str(value)
            elif isinstance(value, datetime.timedelta):
                value = value
            else:
                raise Exception(f"Invalid type {type(value)} for {key}")

        super().__setattr__(key, value)

    @property
    def key_feature(self):
        if self._key_feature is None:
            return self.features[0]
        return self._key_feature

    @key_feature.setter
    def key_feature(self, value):
        self._key_feature = value

    def manifest(self):
        """return a Kubernetes YAML manifest for this FeatureSet definition"""
        return yaml.dump(self, sort_keys=False, Dumper=RaptorDumper)

    @classmethod
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'FeatureSetSpec'):
        data.annotations['a8r.io/description'] = data.description
        manifest = {
            "apiVersion": "k8s.raptor.ml/v1alpha1",
            "kind": "FeatureSet",
            "metadata": {
                "name": _k8s_name(data.name),
                "namespace": data.namespace,
                "labels": data.labels,
                "annotations": data.annotations
            },
            "spec": {
                "timeout": data.timeout,
                "features": data.features,
                "keyFeature": None if data.key_feature == data.features[0] else data.key_feature
            }
        }
        return dumper.represent_mapping(cls.yaml_tag, manifest, flow_style=cls.yaml_flow_style)


RaptorDumper.add_representer(FeatureSetSpec, FeatureSetSpec.to_yaml)


class Keys(Dict[str, str]):
    def encode(self, spec: FeatureSpec) -> str:
        ret = ""
        for key in spec.keys:
            val = self.get(key)
            if val is None:
                raise Exception(f"missing key {key}")
            ret += val + "."
        return ret

    def decode(self, spec: FeatureSpec, encoded_keys: str) -> 'Keys':
        parts = encoded_keys.split(".")
        if len(parts) != len(spec.keys):
            raise Exception(f"invalid key {encoded_keys}")
        for i, encoded_keys in enumerate(spec.keys):
            self[encoded_keys] = parts[i]
        return self

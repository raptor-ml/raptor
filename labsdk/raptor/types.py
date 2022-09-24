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
import inspect
import re
import types
from enum import Enum
from typing import Optional

import pandas as pd
import redbaron
import yaml
from pandas.core.window import RollingGroupby
from redbaron import RedBaron

from . import durpy
from .config import default_namespace
from .pyexp import pyexp
from .yaml import RaptorDumper


# PyExp
class PyExpProgram:
    frame: types.FrameType = None
    f_lineno: int = -1
    code: str = None
    name: str = None

    def __init__(self, func, fqn):
        if not callable(func):
            raise Exception("func must be callable")

        # take a snapshot of the func frame
        try:
            raise Exception()
        except Exception as e:
            self.frame = e.__traceback__.tb_frame.f_back.f_back.f_back
            pass
        self.f_lineno = self.frame.f_lineno

        root_node = RedBaron(inspect.getsource(func))
        if len(root_node) != 1:
            raise RuntimeError("PyExpProgram in LabSDK only supports one function definition")
        node = root_node[0]
        if not isinstance(node, redbaron.DefNode):
            raise RuntimeError("PyExpProgram in LabSDK only supports function definition")
        if node.arguments:
            for arg in node.arguments:
                arg.annotation = ''
        if len(node.decorators) > 0:
            node.decorators = []

        for comp in node.find_all('comparison'):
            if str(comp.value) == 'is':
                comp.value = '=='
            elif str(comp.value) == 'is not':
                comp.value = '!='

        self.code = root_node.dumps().strip()
        self.name = func.__name__
        self.fqn = fqn

        try:
            self.runtime = pyexp.New(self.code, self.fqn)
        except RuntimeError as e:
            raise _wrap_exception(e, self)


class PyExpException(RuntimeError):
    def __int__(self, *args, **kwargs):
        Exception.__init__(*args, **kwargs)


def _wrap_exception(e: Exception, program: PyExpProgram, *args, **kwargs):
    frame_str = re.match(r".*<pyexp>:([0-9]+):([0-9]+)?: (.*)", str(e).replace("\n", ""), flags=re.MULTILINE)
    if frame_str is None or not isinstance(program, PyExpProgram):
        return e
    else:
        err_str = re.match(r"in (.*)Error in ([aA0-zZ09_]+): (.*)", frame_str.group(3))
        if err_str is None:
            err_str = frame_str.group(3).strip()
        else:
            err_str = f"Error in {err_str.group(2)}: {err_str.group(3).strip()}"
        frame = program.frame
        loc = program.f_lineno + int(frame_str.group(1)) - 1
        tb = types.TracebackType(tb_next=None,
                                 tb_frame=frame,
                                 tb_lasti=int(frame_str.group(2)),
                                 tb_lineno=loc)
        return PyExpException(
            f"on {program.name}:\n    {err_str}\n\n️Friendly tip: remember that PyExp is not python3 😬") \
            .with_traceback(tb)


def normalize_fqn(fqn):
    if "." in fqn:
        return fqn
    if "[" in fqn:
        return f"{fqn[:fqn.index('[')]}.{default_namespace}{fqn[fqn.index('['):]}"
    return f"{fqn}.{default_namespace}"


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
    Timestamp = 'timestamp'
    StringList = '[]string'
    IntList = '[]int'
    FloatList = '[]float'
    TimestampList = '[]timestamp'
    Headless = 'headless'

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
        elif p == 'timestamp' or p == datetime:
            return Primitive.Timestamp
        elif p == '[]string' or p == [str]:
            return Primitive.StringList
        elif p == '[]int' or p == [int]:
            return Primitive.IntList
        elif p == '[]float' or p == [float]:
            return Primitive.FloatList
        elif p == '[]timestamp' or p == [datetime]:
            return Primitive.TimestampList
        elif p == 'headless':
            return Primitive.Headless
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

    connector: ResourceReference = None
    builder: BuilderSpec = BuilderSpec(None)
    aggr: AggrSpec = None

    program: PyExpProgram = None

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
            if isinstance(value, PyExpProgram):
                pass
            elif callable(value):
                value = PyExpProgram(value, self.fqn())
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
            return f"{self.name}.{default_namespace}"
        return f"{self.name}.{self.namespace}"

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
            data.builder.aggr_granularity = data.aggr.granularity
        data.builder.pyexp = data.program.code

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
                "connector": None if data.connector is None else data.connector.__dict__,
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
            return f"{self.name}.{default_namespace}"
        return f"{self.name}.{self.namespace}"

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

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

import os
import re
from enum import Enum
from typing import Optional, Dict, List

import yaml

from .yaml import RaptorDumper
from .. import default_namespace


def _k8s_name(name):
    name = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    name = re.sub('__([A-Z])', r'_\1', name)
    name = re.sub('([a-z0-9])([A-Z])', r'\1_\2', name)
    return name.replace('_', '-').lower()


class RaptorSpec(yaml.YAMLObject):
    name: str = None
    _namespace: str = None
    description: str = None
    labels: dict = {}
    annotations: dict = {}

    def __init__(self, name, namespace=None, description=None, labels=None, annotations=None):
        self.name = name
        self._namespace = namespace
        self.description = description or ''
        self.labels = labels or {}
        self.annotations = annotations or {}

    @classmethod
    def yaml_tag(cls):
        return f'!{cls.__class__.__name__}'

    def fqn(self):
        return f'{self.namespace}.{self.name}'.lower()

    @property
    def namespace(self):
        return self._namespace or default_namespace

    @namespace.setter
    def namespace(self, value):
        self._namespace = value

    def __str__(self):
        return self.__repr__()

    def __repr__(self):
        return f'{self.__class__.name}({self.fqn()})'

    def manifest_filename(self):
        """
        Returns the filename of the manifest file exportation
        :return: the filename
        """
        typ = self.__class__.__name__.replace('Spec', '').replace('Impl', '').lower()
        base_dir = os.path.join(os.getcwd(), 'out')
        return os.path.join(base_dir, f'{typ}.{self.fqn().lower()}.yaml')

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
            filename = self.manifest_filename()
            basename = os.path.dirname(filename)
            if not os.path.exists(basename):
                os.makedirs(basename)

            with open(filename, 'w') as f:
                f.write(ret)
        return ret

    @classmethod
    def to_yaml_dict(cls, data) -> dict:
        return data.__dict__

    @classmethod
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'RaptorSpec'):
        return dumper.represent_mapping(data.yaml_tag(), data.to_yaml_dict(data), flow_style=data.yaml_flow_style)


RaptorDumper.add_multi_representer(RaptorSpec, RaptorSpec.to_yaml)


class EnumSpec(Enum):
    @classmethod
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data):
        return dumper.represent_scalar(f'!{cls.__name__}', data.value)


RaptorDumper.add_multi_representer(EnumSpec, EnumSpec.to_yaml)


class RuntimeSpec(yaml.YAMLObject):
    runtime: Optional[str] = None
    packages: Optional[List[str]] = None

    def __init__(self, runtime: Optional[str] = None, packages: Optional[List[str]] = None):
        self.runtime = runtime
        self.packages = packages


class ResourceReference(yaml.YAMLObject):
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


class SecretKeyRef(yaml.YAMLObject):
    name: str = None
    key: str = None

    def __init__(self, name: str, key: str):
        self.name = name
        self.key = key


class ConfigVar(yaml.YAMLObject):
    name: str = None
    value: Optional[str] = None
    secretKeyRef: Optional[SecretKeyRef] = None

    def __init__(self, name, value=None, secret_key_ref=None):
        self.name = name
        self.value = value
        self.secretKeyRef = secret_key_ref
        if value is None and secret_key_ref is None:
            raise Exception('Must specify either value or secretKeyRef')


class ResourceRequirements(yaml.YAMLObject):
    limits: Dict[str, str] = {}
    requests: Dict[str, str] = {}

    def __init__(self, limits=None, requests=None):
        self.limits = limits or {}
        self.requests = requests or {}

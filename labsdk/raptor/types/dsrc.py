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

from typing import List, Optional, Dict, Any

import pandas as pd

from .common import _k8s_name, RaptorSpec, ResourceRequirements, SecretKeyRef
from .dsrc_config_stubs.protocol import SourceProductionConfig
from .._internal.exporter.general import GeneralExporter


class DataSourceSpec(RaptorSpec):
    production_config: SourceProductionConfig = None,
    schema: Optional[Dict[str, Any]] = None
    keys: List[str] = None
    timestamp: str = None
    replicas: int = None
    resources: ResourceRequirements = None

    features: List[RaptorSpec] = []

    local_df: pd.DataFrame = None

    def __init__(self, name, keys=None, timestamp=None, production_config: Optional[SourceProductionConfig] = None,
                 *args, **kwargs):
        super().__init__(name, *args, **kwargs)
        self.keys = keys or []
        self.timestamp = timestamp
        if production_config is None:
            production_config = SourceProductionConfig()
        self.production_config = production_config

    def export(self):
        GeneralExporter.add_source(self)
        GeneralExporter.export()

    @classmethod
    def to_yaml_dict(cls, data: 'DataSourceSpec'):
        data.annotations['a8r.io/description'] = data.description

        config = []

        if data.production_config is not None:
            for k, v in data.production_config.configurable_envs().items():
                GeneralExporter.add_env(k, v)

            for k, v in data.production_config.config().items():
                if isinstance(v, SecretKeyRef):
                    config.append({'name': k, 'secretKeyRef': {'name': v.name, 'key': v.key}})
                else:
                    config.append({'name': k, 'value': v})

        return {
            'apiVersion': 'k8s.raptor.ml/v1alpha1',
            'kind': 'DataSource',
            'metadata': {
                'name': _k8s_name(data.name),
                'namespace': data.namespace,
                'labels': data.labels,
                'annotations': data.annotations
            },
            'spec': {
                'kind': data.production_config.kind(),
                'config': config,
                'keyFields': data.keys,
                'timestampField': data.timestamp,
                'replicas': data.replicas,
                'resources': data.resources,
                'schema': data.schema
            }
        }

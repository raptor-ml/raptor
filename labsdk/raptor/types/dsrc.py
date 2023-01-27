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

from .common import _k8s_name, RaptorSpec, ConfigVar, ResourceRequirements
from .._internal.exporter.general import GeneralExporter


class DataSourceSpec(RaptorSpec):
    kind: str = None
    config: List[ConfigVar] = None
    schema: Optional[Dict[str, Any]] = None
    keys: List[str] = None
    timestamp: str = None
    replicas: int = None
    resources: ResourceRequirements = None

    features: List[RaptorSpec] = []

    _local_df: pd.DataFrame = None

    def __init__(self, name, keys=None, timestamp=None, *args, **kwargs):
        super().__init__(name, *args, **kwargs)
        self.keys = keys or []
        self.timestamp = timestamp

    def export(self):
        GeneralExporter.add_source(self)
        GeneralExporter.export()

    @classmethod
    def to_yaml_dict(cls, data: 'DataSourceSpec'):
        data.annotations['a8r.io/description'] = data.description
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
                'kind': data.kind,
                'config': data.config,
                'keyFields': data.keys,
                'timestampField': data.timestamp,
                'replicas': data.replicas,
                'resources': data.resources,
                'schema': data.schema
            }
        }

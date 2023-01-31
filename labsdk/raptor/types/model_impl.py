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

import inspect
from datetime import timedelta
from typing import Optional, Callable

from . import SecretKeyRef
from .common import _k8s_name
from .model import ModelSpec, TrainingContext
from .. import local_state, replay, durpy
from .._internal.exporter import ModelExporter
from .._internal.exporter.general import GeneralExporter


class ModelImpl(ModelSpec):
    _key_feature: str = None
    _model_filename: Optional[str] = None
    _features_and_labels: Callable = None

    def __init__(self, keys=None, model_framework=None, model_server=None, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.keys = keys
        self.model_framework = model_framework
        self.model_server = model_server

        self.exporter = ModelExporter(self)
        self._features_and_labels = replay.new_historical_get(self)

    def features_and_labels(self):
        return self._features_and_labels()

    def train(self):
        for f in (self.features + self.label_features + ([self.key_feature] if self.key_feature is not None else [])):
            s = local_state.feature_spec_by_selector(f)
            replay.new_replay(s)()

        model = self.training_function(TrainingContext(
            keys=self.keys,
            input_labels=self.label_features,
            input_features=self.features,
            data_getter=self.features_and_labels,
        ))
        self.exporter.save(model)

        return model

    def export(self, with_dependent_features=True, with_dependent_sources=True):
        GeneralExporter.add_model(self, with_dependent_features=with_dependent_features,
                                  with_dependent_sources=with_dependent_sources)
        GeneralExporter.export()

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
            return self.features[0]
        return self._key_feature

    @key_feature.setter
    def key_feature(self, value):
        self._key_feature = value

    @classmethod
    def to_yaml_dict(cls, data: 'ModelImpl'):
        inference_config_stub = []

        if data.model_server is not None and data.model_server.config is not None:
            for k, v in data.model_server.config.configurable_envs().items():
                GeneralExporter.add_env(k, v)

            for k, v in data.model_server.config.inference_config().items():
                if isinstance(v, SecretKeyRef):
                    inference_config_stub.append({'name': k, 'secretKeyRef': {'name': v.name, 'key': v.key}})
                else:
                    inference_config_stub.append({'name': k, 'value': v})

        data.annotations['a8r.io/description'] = data.description

        if data._model_filename is not None:
            GeneralExporter.add_env('MODEL_STORAGE_BASE_URI', '(REQUIRED) The base URI for the model storage'
                                                              ' (e.g. s3://bucket-name/all_models)')

        if data._model_tag is not None:
            GeneralExporter.add_env('MODEL_IMAGE_REPO_URI', '(REQUIRED) The URI for the model image repository'
                                                            '(e.g. 123456789012.dkr.ecr.us-west-2.amazonaws.com/my'
                                                            '-model-repo)')

        return {
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
                'keys': data.keys,
                'modelFramework': data.model_framework,
                'modelFrameworkVersion': data.model_framework_version,
                'modelServer': data.model_server,
                'inferenceConfig': inference_config_stub,
                'storageURI': None if data._model_filename is None else f'$MODEL_STORAGE_BASE_URI/'
                                                                        f'{data._model_filename}',
                'modelImage': None if data._model_tag is None else f'$MODEL_IMAGE_REPO_URI:{data._model_tag}',
                'trainingCode': inspect.getsource(data.training_function),
            }
        }

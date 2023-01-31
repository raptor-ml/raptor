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

import os
import shutil
import tempfile
from warnings import warn

import bentoml
import bentoml.bentos
from attr import evolve
from bentoml import Bento
from bentoml._internal.bento.build_config import BentoBuildConfig
from bentoml.exceptions import NotFound
from pandas import __version__ as pandas_version

from ...types.model import ModelFramework, ModelSpec


class ModelExporter:
    spec: ModelSpec = None
    model: bentoml.Model = None

    def __init__(self, spec: ModelSpec):
        self.spec = spec

    def get_model(self):
        if self.model is not None:
            return self.model

        # check if out/models/fqn exists
        # if so, load it
        model_path = os.path.join(os.getcwd(), 'out', 'models', self.spec.fqn(), 'models', self.spec.fqn())
        if os.path.exists(os.path.join(model_path, 'latest')):
            with open(os.path.join(model_path, 'latest'), 'r') as f:
                version = f.read()
                try:
                    self.model = bentoml.models.get(f'{self.spec.fqn()}:{version}')
                except NotFound:
                    self.model = bentoml.models.import_model(os.path.join(model_path, version), 'folder')

                return self.model

        return None

    def save(self, model):
        opts = {}
        if self.spec.model_framework == ModelFramework.HuggingFace:
            self.model = bentoml.transformers.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.Sklearn:
            self.model = bentoml.sklearn.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.Pytorch:
            self.model = bentoml.pytorch.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.PytorchLightning:
            self.model = bentoml.pytorch_lightning.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.Tensorflow:
            self.model = bentoml.tensorflow.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.CatBoost:
            self.model = bentoml.catboost.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.ONNX:
            self.model = bentoml.onnx.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.LightGBM:
            self.model = bentoml.lightgbm.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.XGBoost:
            self.model = bentoml.xgboost.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.FastAI:
            self.model = bentoml.fastai.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.Keras:
            self.model = bentoml.keras.save_model(self.spec.fqn(), model, **opts)
        elif self.spec.model_framework == ModelFramework.Picklable:
            self.model = bentoml.picklable_model.save_model(self.spec.fqn(), model, **opts)
        else:
            warn(f'ModelFramework {self} not deployable by Raptor at the moment.'
                 'Please export your model manually instead')

        if self.spec.runtime.packages is None:
            self.spec.runtime.packages = []
        self.spec.runtime.packages.append(f'pandas=={pandas_version}')
        for k, v in self.model.info.context.framework_versions.items():
            if self.spec.model_framework_version is None or self.spec.model_framework_version == 'unknown':
                self.spec.model_framework_version = v
            self.spec.runtime.packages.append(f'{k}=={v}')

    def create_docker(self, remove_unused_models=True):
        if self.get_model() is None:
            raise Exception('Model not trained yet')

        cwd = os.getcwd()
        with tempfile.TemporaryDirectory() as tmpdir, open(os.path.join(tmpdir, 'service.py'), 'w') as f:
            f.write(f'''
from typing import Dict, Any

import bentoml
import pandas as pd
from bentoml.io import JSON

model_runner = bentoml.models.get('{self.model.tag}').to_runner()
svc = bentoml.Service("{self.spec.fqn()}", runners=[model_runner])


@svc.api(input=JSON(), output=JSON())
def predict(input_data: Dict[str, Any]) -> Dict[str, Any]:
    return model_runner.run(pd.DataFrame([input_data]))
    '''.strip('\n'))
            f.flush()

            labels = self.spec.labels.copy()
            labels['created_by'] = 'raptor-labsdk'

            build_config = BentoBuildConfig(
                description=self.spec.description,
                labels=self.spec.labels,
                service=os.path.basename(f.name),
            ).with_defaults()

            packages = build_config.python.packages or []
            build_config = evolve(build_config,
                                  python=evolve(build_config.python, packages=packages + self.spec.runtime.packages))

            if self.spec.model_server is not None and self.spec.model_server.config is not None:
                build_config = self.spec.model_server.config.apply_bento_config(build_config)

            bento = Bento.create(
                build_config=build_config,
                version=self.model.tag.version,
                build_ctx=os.path.dirname(f.name),
            ).save()
            os.chdir(cwd)  # bentos.build changes cwd. change it back to avoid clashes when we export other outputs

            if self.spec.model_server is not None and self.spec.model_server.config is not None:
                self.spec.model_server.config.post_build(bento)

            models_dir = os.path.join(cwd, 'out', 'models')
            base_dir = os.path.join(models_dir, self.spec.fqn())
            if not os.path.exists(base_dir):
                os.makedirs(base_dir)

            lfs_attrs = os.path.join(base_dir, '.gitattributes')
            if not os.path.exists(lfs_attrs):
                with open(lfs_attrs, 'w') as af:
                    af.write('*/models/*/*/saved_model.* filter=lfs diff=lfs merge=lfs -text')
                    af.flush()

            shutil.copytree(bento.path, base_dir, dirs_exist_ok=True)

            if remove_unused_models:
                models_models_dir = os.path.join(base_dir, 'models', self.model.tag.name)
                for d in os.listdir(models_models_dir):
                    if d != self.model.tag.version and os.path.isdir(os.path.join(models_models_dir, d)):
                        shutil.rmtree(os.path.join(models_models_dir, d))

            self.spec._model_tag = str(bento.tag.version)

    def export(self, with_docker=True, remove_unused_models=True):
        if self.model is None:
            self.spec.train()

        if with_docker:
            self.create_docker(remove_unused_models=remove_unused_models)

        return self.spec.manifest(True)

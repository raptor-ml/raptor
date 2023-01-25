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
from bentoml import Bento
from bentoml._internal.bento.build_config import BentoBuildConfig

from ... import local_state
from ...types.model import ModelFramework, ModelSpec


class ModelExporter:
    spec: ModelSpec = None
    model: bentoml.Model = None

    def __init__(self, spec: ModelSpec):
        self.spec = spec

    def save(self, model):
        opts = {
            'signatures': {'__call__': {'batchable': False}}
        }
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

    def create_docker(self):
        if self.model is None:
            raise Exception('Model not trained yet')

        cwd = os.getcwd()
        with tempfile.TemporaryDirectory() as tmpdir, open(os.path.join(tmpdir, 'service.py'), 'w') as f:
            f.write(f'''
import numpy as np
import bentoml
from bentoml.io import JSON

model_runner = bentoml.models.get('{self.model.tag}').to_runner()
svc = bentoml.Service("{self.spec.fqn()}", runners=[model_runner])

@svc.api(input=JSON(), output=JSON())
def classify(input_series: np.ndarray) -> np.ndarray:
    return model_runner.predict.run(input_series)
    '''.strip('\n'))
            f.flush()

            labels = self.spec.labels.copy()
            labels['created_by'] = 'raptor-labsdk'

            build_config = BentoBuildConfig(
                description=self.spec.description,
                labels=self.spec.labels,
                service=os.path.basename(f.name),
            ).with_defaults()

            if self.spec.model_server is not None and self.spec.model_server.config is not None:
                build_config = self.spec.model_server.config.apply_bento_config(build_config)

            bento = Bento.create(
                build_config=build_config,
                version=None,
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
            self.spec._model_image = bento.tag.version

    def export(self, with_dependent_features: bool = True, with_docker=True):
        if self.model is None:
            self.spec.train()

        if with_docker:
            self.create_docker()

        if with_dependent_features:
            for feature in self.spec.features:
                local_state.spec_by_selector(selector=feature).manifest(to_file=True)

        return self.spec.manifest(True)

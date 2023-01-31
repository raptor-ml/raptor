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
from datetime import timedelta
from typing import List, Callable, Optional
from warnings import warn

import pandas as pd

from .common import RaptorSpec, EnumSpec, RuntimeSpec
from .._internal import model_servers

os.environ.setdefault('BENTOML_HOME', os.path.join(os.path.expanduser('~'), '.raptor', 'bentoml'))


class ModelServer(EnumSpec):
    SageMakerACK = 'sagemaker-ack'
    Seldon = 'seldon'
    KServe = 'kserve'
    VertexAI = 'vertexai'
    MLFlow = 'mlflow'

    @staticmethod
    def parse(m):
        if isinstance(m, ModelServer):
            return m
        if isinstance(m, str):
            for ms in ModelServer:
                if ms.value == m:
                    return ms
        raise Exception(f'Unknown ModelServer {m}')

    def __str__(self):
        if self != ModelServer.SageMakerACK:
            warn(f'ModelServer {self} is not supported yet')
        return self.value

    @classmethod
    def to_yaml_dict(cls, data: 'ModelServer'):
        return data.value

    @property
    def config(self) -> Optional[model_servers.ModelServer]:
        if self == ModelServer.SageMakerACK:
            return model_servers.Sagemaker

        warn(f'ModelServer {self} is not supported yet')
        return None


class ModelFramework(EnumSpec):
    HuggingFace = 'huggingface'
    Sklearn = 'sklearn'
    Pytorch = 'pytorch'
    PytorchLightning = 'pytorch-lightning'
    Tensorflow = 'tensorflow'
    CatBoost = 'catboost'
    ONNX = 'onnx'
    LightGBM = 'lightgbm'
    XGBoost = 'xgboost'
    FastAI = 'fastai'
    Keras = 'keras'
    Picklable = 'picklable'

    @staticmethod
    def parse(m):
        if isinstance(m, ModelFramework):
            return m
        if isinstance(m, str):
            for e in ModelFramework:
                if e.value == m:
                    return e
        raise Exception(f'Unknown ModelFramework {m}')


class TrainingContext:
    """
    Context of the model request.
    """

    keys: List[str] = []
    input_features: List[str] = []
    input_labels: List[str] = []
    _data_getter: Callable[[], pd.DataFrame] = None

    def __init__(self, keys: List[str], input_features: List[str], input_labels: List[str],
                 data_getter: Callable[[], pd.DataFrame]):
        self.keys = keys
        self.input_features = input_features
        self.input_labels = input_labels
        self._data_getter = data_getter

    def features_and_labels(self) -> pd.DataFrame:
        """
        Get the features and labels for the model
        """
        return self._data_getter()


class ModelSpec(RaptorSpec):
    keys: List[str] = None
    freshness: Optional[timedelta] = None
    staleness: timedelta = None
    timeout: timedelta = None
    features: List[str] = None
    label_features: List[str] = None

    key_feature: str = None

    model_framework: ModelFramework = None
    model_server: Optional[ModelServer] = None

    training_function: Callable = None
    # noinspection PyUnresolvedReferences
    exporter: 'ModelExporter' = None

    model_framework_version: str = 'unknown'

    runtime: RuntimeSpec = RuntimeSpec(packages=[])

    _model_tag: str = None

    def train(self):
        raise NotImplementedError()

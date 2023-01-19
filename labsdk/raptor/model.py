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
from enum import Enum
from typing import List, Callable
from warnings import warn

import pandas as pd
import yaml

from .frameworks.huggingface import HuggingFaceFramework
from .frameworks.sklearn import SklearnFramework
from .frameworks.xgboost import XGBoostFramework
from .yaml import RaptorDumper


class ModelServer(Enum):
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
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'ModelServer'):
        return dumper.represent_scalar('!ModelServer', data.value)

    @property
    def config(self):
        if self == ModelServer.SageMakerACK:
            return {
                'region': '$AWS_REGION',
                'executionRoleARN': '$AWS_EXECUTION_ROLE_ARN',
            }
        return {}


RaptorDumper.add_representer(ModelServer, ModelServer.to_yaml)


class ModelFramework(Enum):
    HuggingFace = 'huggingface'
    Sklearn = 'sklearn'
    Pytorch = 'pytorch'
    Tensorflow = 'tensorflow'
    CatBoost = 'catboost'
    ONNX = 'onnx'
    LightGBM = 'lightgbm'
    XGBoost = 'xgboost'
    FastAI = 'fastai'

    @staticmethod
    def parse(m):
        if isinstance(m, ModelFramework):
            return m
        if isinstance(m, str):
            for e in ModelFramework:
                if e.value == m:
                    return e
        raise Exception(f'Unknown ModelFramework {m}')

    def save(self, model, spec):
        if self == ModelFramework.HuggingFace:
            return HuggingFaceFramework.save(model, spec)
        elif self == ModelFramework.Sklearn:
            return SklearnFramework.save(model, spec)
        elif self == ModelFramework.XGBoost:
            return XGBoostFramework.save(model, spec)
        warn(f'ModelFramework {self} not Deployable by Raptor at the moment. Please export your model manually instead')

    @classmethod
    def to_yaml(cls, dumper: yaml.dumper.Dumper, data: 'ModelFramework'):
        return dumper.represent_scalar('!ModelFramework', data.value)


RaptorDumper.add_representer(ModelFramework, ModelFramework.to_yaml)


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

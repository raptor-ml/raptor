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
from .base import BaseModelFramework


class HuggingFaceFramework(BaseModelFramework):
    @staticmethod
    def save(model, spec: 'ModelSpec'):
        try:
            from transformers import pipelines, TFPreTrainedModel, PreTrainedModel, __version__ as transformers_version
        except ImportError:
            raise ImportError('Please install transformers to use the huggingface framework.\n'
                              'You can install it using pip: `pip install transformers`')

        if not (False
                or isinstance(model, pipelines.Pipeline)
                or isinstance(model, PreTrainedModel)
                or isinstance(model, TFPreTrainedModel)
        ):
            raise TypeError(
                'model must be a supported HuggingFace model (Pipeline, PreTrainedModel, TFPreTrainedModel)')

        fw_ver = ''
        if model.framework == 'pt':
            try:
                from torch import __version__ as torch_version
                fw_ver = 'pytorch' + torch_version
            except ImportError:
                raise ImportError('Please install torch to train this model.\n'
                                  'You can install it using pip: `pip install torch`')
        elif model.framework == 'tf':
            try:
                from tensorflow import __version__ as tf_version
                fw_ver = 'tensorflow' + tf_version
            except ImportError:
                raise ImportError('Please install tensorflow to train this model.\n'
                                  'You can install it using pip: `pip install tensorflow`')
        else:
            raise ValueError('Unknown framework')

        BaseModelFramework._create_output_path()

        spec._model_framework_version = f'{transformers_version}+{fw_ver}'

        base_filename = f'{spec.fqn()}_{model.__hash__()}'
        model_path = f'{BaseModelFramework._base_output_path()}/{base_filename}'
        model.save_pretrained(model_path)

        import shutil
        shutil.make_archive(model_path, 'gztar', model_path)
        shutil.rmtree(model_path)
        spec._model_filename += f'{base_filename}.tar.gz'

    @staticmethod
    def predict(model, data):
        return model(data)

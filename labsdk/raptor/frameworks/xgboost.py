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


class XGBoostFramework(BaseModelFramework):
    @staticmethod
    def save(model, spec: 'ModelSpec'):
        try:
            import joblib
            from xgboost import __version__ as xgboost_version
            from xgboost import Booster
        except ImportError:
            raise ImportError('Please install xgboost to use the xgboost framework.\n'
                              'You can install them using pip: `pip install xgboost`')

        if not isinstance(model, Booster):
            raise TypeError('model must be a sklearn model')

        BaseModelFramework._create_output_path()
        spec._model_filename = f'{spec.fqn()}_{model.__hash__()}.ubj'
        spec._model_framework_version = xgboost_version

        base_filename = f'{spec.fqn()}_{model.__hash__()}.ubj'
        model_path = f'{BaseModelFramework._base_output_path()}/{base_filename}'
        model.save_model(model_path)

        import shutil, os
        shutil.make_archive(model_path, 'gztar', BaseModelFramework._base_output_path())
        os.remove(model_path)
        spec._model_filename = f'{base_filename}.tar.gz'

    @staticmethod
    def predict(model, data):
        return model.predict(data)

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
from typing import Optional


class BaseModelFramework:
    @staticmethod
    def save(model, spec: 'ModelSpec'):
        raise NotImplementedError

    @staticmethod
    def predict(model, spec: 'ModelSpec'):
        raise NotImplementedError


    @staticmethod
    def _base_output_path():
        return f"out/models"

    @staticmethod
    def _create_output_path(path: Optional[str] = None):
        if path is None:
            path = BaseModelFramework._base_output_path()
        if not os.path.exists(path):
            os.makedirs(path)
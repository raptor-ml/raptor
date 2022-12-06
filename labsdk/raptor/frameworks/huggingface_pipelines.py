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


class HuggingFacePipelinesFramework(BaseModelFramework):
    @staticmethod
    def save(model: 'pipelines.Pipeline', spec: 'ModelSpec'):
        try:
            from transformers import pipelines
        except ImportError:
            raise ImportError("Please install transformers to use huggingface pipelines framework")

        if not isinstance(model, pipelines.Pipeline):
            raise TypeError("model must be a HuggingFace Pipeline")

        BaseModelFramework._create_output_path()
        model.save_pretrained(f"{BaseModelFramework._base_output_path()}/{spec.fqn()}")

    @staticmethod
    def predict(model, data):
        return model(data)
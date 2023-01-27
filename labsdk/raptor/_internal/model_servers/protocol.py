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

from typing import Dict, Union

from bentoml import Bento
from bentoml._internal.bento.build_config import BentoBuildConfig
from typing_extensions import Protocol

from ...types.common import SecretKeyRef


class ModelServer(Protocol):
    @classmethod
    def configurable_envs(cls) -> Dict[str, str]:
        return {}

    @classmethod
    def inference_config(cls, **kwargs) -> Dict[str, Union[str, SecretKeyRef]]:
        return {}

    @classmethod
    def apply_bento_config(cls, cfg: BentoBuildConfig) -> BentoBuildConfig:
        return cfg

    @classmethod
    def post_build(cls, bento: Bento):
        pass

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
from typing import Dict, Union

from .protocol import SourceProductionConfig
from ..common import SecretKeyRef


class StreamingKind(Enum):
    Stub = 'stub'
    KAFKA = 'kafka'
    GCP_PUBSUB = 'gcp_pubsub'


class StreamingConfig(SourceProductionConfig):
    brokerKind: StreamingKind = StreamingKind.Stub

    def __init__(self, kind: Union[str, StreamingKind] = 'kafka', *args, **kwargs):
        super().__init__(*args, **kwargs)

        if isinstance(kind, str):
            kind = StreamingKind[kind.upper()]

        self.brokerKind = kind

    def configurable_envs(self) -> Dict[str, str]:
        return {}

    def config(self) -> Dict[str, Union[str, SecretKeyRef]]:
        if self.brokerKind == StreamingKind.GCP_PUBSUB:
            return {
                'kind': 'gcp_pubsub',
                'project_id': '<project_id>',
                'topic': '<topic>',
                'credential_json': SecretKeyRef('gcp-credentials', 'json'),
                'max_batch_size': '100',
            }
        if self.brokerKind == StreamingKind.KAFKA:
            return {
                'kind': 'kafka',
                'brokers': 'localhost:9092',
                'topics': '<topic>',
                'consumer_group': '',
                'client_id': '',
                'sasl_username': '',
                'sasl_password': '',
                'tls_disable': '',
                'tls_skip_verify': '',
                'tls_ca_cert': '',
                'tls_client_cert': '',
                'tls_client_key': '',
                'initial_offset': '',
                'version': '',
            }
        return {
            'kind': 'stub',
        }

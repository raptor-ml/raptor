# -*- coding: utf-8 -*-
# Licensed as Apache2
# File taken from https://github.com/bentoml/aws-sagemaker-deploy/blob/main/bentoctl_sagemaker/sagemaker/service.py

import logging

import bentoml
from bentoml.exceptions import BentoMLException
from starlette.requests import Request
from starlette.types import ASGIApp, Receive, Scope, Send

AWS_SAGEMAKER_SERVE_PORT = 8080
AWS_CUSTOM_ENDPOINT_HEADER = 'X-Amzn-SageMaker-Custom-Attributes'
BENTOML_HEALTH_CHECK_PATH = '/livez'

logger = logging.getLogger(__name__)

# use standalone_load so that the path is not changed back
# after loading.
svc = bentoml.load('.', standalone_load=True)


class SagemakerMiddleware:
    def __init__(
        self,
        app: ASGIApp,
    ) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope['type'] == 'http':
            req = Request(scope, receive)
            if req.url.path == '/ping':
                scope['path'] = BENTOML_HEALTH_CHECK_PATH

            if req.url.path == '/invocations':
                if AWS_CUSTOM_ENDPOINT_HEADER not in req.headers:
                    if len(svc.apis) == 1:
                        # only one api, use it
                        api_path, *_ = svc.apis
                        logger.info(
                            f"'{AWS_CUSTOM_ENDPOINT_HEADER}' not found in request header. Using default {api_path} "
                            f'service.'
                        )
                    else:
                        logger.error(
                            f"'{AWS_CUSTOM_ENDPOINT_HEADER}' not found inside request header. "
                            f'If you are directly invoking the Sagemaker Endpoint pass in the '
                            f"'{AWS_CUSTOM_ENDPOINT_HEADER}' with the bentoml service name that you want to invoke."
                        )
                        raise BentoMLException(
                            f"'{AWS_CUSTOM_ENDPOINT_HEADER}' not found inside request header."
                        )
                else:
                    api_path = req.headers[AWS_CUSTOM_ENDPOINT_HEADER]
                    if api_path not in svc.apis:
                        logger.error(
                            "API Service passed via the '{AWS_CUSTOM_ENDPOINT_HEADER}' not found in the bentoml "
                            'service.'
                        )
                        raise BentoMLException(
                            "API Service passed via the '{AWS_CUSTOM_ENDPOINT_HEADER}' not found in the bentoml "
                            'service.'
                        )
                scope['path'] = '/' + api_path

        await self.app(scope, receive, send)


# noinspection PyTypeChecker
svc.add_asgi_middleware(SagemakerMiddleware)

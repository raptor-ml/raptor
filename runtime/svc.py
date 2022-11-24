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
import subprocess
import warnings
from datetime import datetime
from uuid import uuid4

import grpc
from google.protobuf.internal.containers import MessageMap
from grpc import ServicerContext
from typing import Dict

from program import Program, Context

import sys
sys.path.append("./proto")

from proto.raptor.core.v1alpha1 import api_pb2 as core_pb2
from proto.raptor.core.v1alpha1 import api_pb2_grpc as core_grpc
from proto.raptor.core.v1alpha1 import types_pb2 as types_pb2
from proto.raptor.runtime.v1alpha1 import api_pb2
from proto.raptor.runtime.v1alpha1 import api_pb2_grpc

class RuntimeServicer(api_pb2_grpc.RuntimeServiceServicer):
    programs: Dict[str, Program] = {}
    engine: core_grpc.EngineServiceStub

    def __init__(self, engine_channel: grpc.aio.Channel):
        self.engine = core_grpc.EngineServiceStub(engine_channel)

    def attach_to_server(self, server):
        api_pb2_grpc.add_RuntimeServiceServicer_to_server(self, server)

    @staticmethod
    def full_name():
        return api_pb2.DESCRIPTOR.services_by_name[api_pb2_grpc.RuntimeService.__name__].full_name

    async def LoadProgram(self, request: api_pb2.LoadProgramRequest, context: ServicerContext):
        try:
            for pkg in request.packages:
                subprocess.run([sys.executable, "-m", "pip", "install", pkg], check=True)

            program = Program(request.program)
            self.programs[request.fqn] = program
            return api_pb2.LoadProgramResponse(uuid=request.uuid)
        except Exception as e:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(e))
            return

    async def RegisterSchema(self, request: api_pb2.RegisterSchemaRequest, context: ServicerContext):
        print(request)
        return api_pb2.RegisterSchemaResponse()

    async def ExecuteProgram(self, request: api_pb2.ExecuteProgramRequest, context: ServicerContext):
        if request.fqn not in self.programs:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Program not found")
            return

        program: Program = self.programs[request.fqn]

        keys = {}
        for key, value in request.keys:
            keys[key] = value

        ts = request.timestamp.ToDatetime()

        def feature_getter(fqn: str, keys: Dict[str, str], timestamp: datetime) -> any:
            if timestamp != ts:
                warnings.warn("Timestamp mismatch")
            fg_keys = keys if keys is not None else {}
            req = core_pb2.GetRequest(uuid=str(uuid4()), fqn=fqn, keys=fg_keys)
            resp: core_pb2.GetResponse = self.engine.Get(req)
            if resp.uuid != req.uuid:
                raise Exception("UUID mismatch")

            return self.proto_value_to_py(resp.value)

        data = self.proto_to_dict(request.data)
        program_ctx = Context(
            fqn=request.fqn,
            keys=keys,
            timestamp=ts,
            feature_getter=feature_getter,
        )

        try:
            result = program.call(data, program_ctx)
            return api_pb2.ExecuteProgramResponse(result=result)
        except Exception as e:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(e))
            return

    @staticmethod
    def proto_to_dict(self, data: MessageMap[str, types_pb2.Value]) -> Dict[str, any]:
        result = {}
        for k, v in data.items():
            result[k] = self.proto_value_to_py(v)
        return result

    @staticmethod
    def proto_value_to_py(self, value: types_pb2.Value) -> any:
        if value.scalar_value is not None:
            return self.proto_scalar_to_py(value.scalar_value)
        elif value.list_value is not None:
            return [self.proto_scalar_to_py(scalar) for scalar in value.list_value.values]
        return None

    @staticmethod
    def proto_scalar_to_py(self, scalar: types_pb2.Scalar) -> any:
        if scalar.int_value is not None:
            return scalar.int_value
        elif scalar.float_value is not None:
            return scalar.float_value
        elif scalar.string_value is not None:
            return scalar.string_value
        elif scalar.bool_value is not None:
            return scalar.bool_value
        elif scalar.timestamp_value is not None:
            return scalar.timestamp_value.ToDatetime()
        return None

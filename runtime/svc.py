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
import logging
import subprocess
import warnings
from datetime import datetime
from uuid import uuid4

import grpc
from google.protobuf.internal.containers import MessageMap
from grpc import ServicerContext
from typing import Dict, List

from program import Program, Context, SideEffect

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
            return api_pb2.LoadProgramResponse(uuid=request.uuid,
                                               primitive=self.py_to_proto_primitive(program.primitive),
                                               side_effects=self.py_to_proto_side_effects(program.side_effects))
        except Exception as e:
            logging.error(f"{request.fqn}: Failed to load program", e)
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(e))
            return

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
            fg_keys = keys if keys is not None else request.keys
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
            resp = program.call(data, program_ctx)
            if isinstance(resp, tuple) and len(resp) == 3:
                if not isinstance(resp[2], datetime):
                    raise Exception("Timestamp must be a datetime object")
                ts = resp[2]
            return api_pb2.ExecuteProgramResponse(
                uuid=request.uuid,
                result=self.py_to_proto_value(resp[0] if isinstance(resp, tuple) else resp),
                keys=resp[1] if isinstance(resp, tuple) else request.keys,
                timestamp=ts,
            )
        except Exception as e:
            logging.error(f"{request.fqn}: Failed to execute program", e)
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

    @staticmethod
    def py_to_proto_value(self, value: any) -> types_pb2.Value:
        if isinstance(value, int):
            return types_pb2.Value(scalar_value=types_pb2.Scalar(int_value=value))
        elif isinstance(value, float):
            return types_pb2.Value(scalar_value=types_pb2.Scalar(float_value=value))
        elif isinstance(value, str):
            return types_pb2.Value(scalar_value=types_pb2.Scalar(string_value=value))
        elif isinstance(value, bool):
            return types_pb2.Value(scalar_value=types_pb2.Scalar(bool_value=value))
        elif isinstance(value, datetime):
            return types_pb2.Value(scalar_value=types_pb2.Scalar(timestamp_value=value))
        elif isinstance(value, list):
            return types_pb2.Value(list_value=types_pb2.List(values=[self.py_to_proto_scalar(v) for v in value]))
        return types_pb2.Value()

    def py_to_proto_primitive(self, primitive) -> types_pb2.Primitive:
        if primitive == str:
            return types_pb2.PRIMITIVE_STRING
        elif primitive == int:
            return types_pb2.PRIMITIVE_INTEGER
        elif primitive == float:
            return types_pb2.PRIMITIVE_FLOAT
        elif primitive == bool:
            return types_pb2.PRIMITIVE_BOOL
        elif primitive == datetime:
            return types_pb2.PRIMITIVE_TIMESTAMP
        elif primitive == List[str]:
            return types_pb2.PRIMITIVE_STRING_LIST
        elif primitive == List[int]:
            return types_pb2.PRIMITIVE_INTEGER_LIST
        elif primitive == List[float]:
            return types_pb2.PRIMITIVE_FLOAT_LIST
        elif primitive == List[bool]:
            return types_pb2.PRIMITIVE_BOOL_LIST
        elif primitive == List[datetime]:
            return types_pb2.PRIMITIVE_TIMESTAMP_LIST
        return types_pb2.PRIMITIVE_UNSPECIFIED

    def py_to_proto_side_effects(self, side_effects: List[SideEffect]) -> List[api_pb2.SideEffect]:
        se = []
        for effect in side_effects:
            se.append(api_pb2.SideEffect(
                kind=effect.kind,
                args=effect.args,
                conditional=effect.conditional,
            ))
        return se

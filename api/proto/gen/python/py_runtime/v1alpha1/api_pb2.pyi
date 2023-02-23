from google.api import annotations_pb2 as _annotations_pb2
from google.api import visibility_pb2 as _visibility_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from core.v1alpha1 import types_pb2 as _types_pb2
from validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SideEffect(_message.Message):
    __slots__ = ["kind", "args", "conditional"]
    class ArgsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KIND_FIELD_NUMBER: _ClassVar[int]
    ARGS_FIELD_NUMBER: _ClassVar[int]
    CONDITIONAL_FIELD_NUMBER: _ClassVar[int]
    kind: str
    args: _containers.ScalarMap[str, str]
    conditional: bool
    def __init__(self, kind: _Optional[str] = ..., args: _Optional[_Mapping[str, str]] = ..., conditional: bool = ...) -> None: ...

class ExecuteProgramRequest(_message.Message):
    __slots__ = ["uuid", "fqn", "keys", "data", "timestamp", "dry_run"]
    class KeysEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class DataEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: _types_pb2.Value
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[_types_pb2.Value, _Mapping]] = ...) -> None: ...
    UUID_FIELD_NUMBER: _ClassVar[int]
    FQN_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    DRY_RUN_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    fqn: str
    keys: _containers.ScalarMap[str, str]
    data: _containers.MessageMap[str, _types_pb2.Value]
    timestamp: _timestamp_pb2.Timestamp
    dry_run: bool
    def __init__(self, uuid: _Optional[str] = ..., fqn: _Optional[str] = ..., keys: _Optional[_Mapping[str, str]] = ..., data: _Optional[_Mapping[str, _types_pb2.Value]] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., dry_run: bool = ...) -> None: ...

class ExecuteProgramResponse(_message.Message):
    __slots__ = ["uuid", "result", "keys", "timestamp"]
    class KeysEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    UUID_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    result: _types_pb2.Value
    keys: _containers.ScalarMap[str, str]
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, uuid: _Optional[str] = ..., result: _Optional[_Union[_types_pb2.Value, _Mapping]] = ..., keys: _Optional[_Mapping[str, str]] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class LoadProgramRequest(_message.Message):
    __slots__ = ["uuid", "fqn", "program", "packages"]
    UUID_FIELD_NUMBER: _ClassVar[int]
    FQN_FIELD_NUMBER: _ClassVar[int]
    PROGRAM_FIELD_NUMBER: _ClassVar[int]
    PACKAGES_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    fqn: str
    program: str
    packages: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, uuid: _Optional[str] = ..., fqn: _Optional[str] = ..., program: _Optional[str] = ..., packages: _Optional[_Iterable[str]] = ...) -> None: ...

class LoadProgramResponse(_message.Message):
    __slots__ = ["uuid", "primitive", "side_effects"]
    UUID_FIELD_NUMBER: _ClassVar[int]
    PRIMITIVE_FIELD_NUMBER: _ClassVar[int]
    SIDE_EFFECTS_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    primitive: _types_pb2.Primitive
    side_effects: _containers.RepeatedCompositeFieldContainer[SideEffect]
    def __init__(self, uuid: _Optional[str] = ..., primitive: _Optional[_Union[_types_pb2.Primitive, str]] = ..., side_effects: _Optional[_Iterable[_Union[SideEffect, _Mapping]]] = ...) -> None: ...

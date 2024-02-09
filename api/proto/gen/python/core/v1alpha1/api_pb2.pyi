from google.api import annotations_pb2 as _annotations_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from core.v1alpha1 import types_pb2 as _types_pb2
from validate import validate_pb2 as _validate_pb2
from protoc_gen_openapiv2.options import annotations_pb2 as _annotations_pb2_1
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetRequest(_message.Message):
    __slots__ = ("uuid", "selector", "keys")
    class KeysEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    UUID_FIELD_NUMBER: _ClassVar[int]
    SELECTOR_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    selector: str
    keys: _containers.ScalarMap[str, str]
    def __init__(self, uuid: _Optional[str] = ..., selector: _Optional[str] = ..., keys: _Optional[_Mapping[str, str]] = ...) -> None: ...

class GetResponse(_message.Message):
    __slots__ = ("uuid", "value", "feature_descriptor")
    UUID_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    FEATURE_DESCRIPTOR_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    value: _types_pb2.FeatureValue
    feature_descriptor: _types_pb2.FeatureDescriptor
    def __init__(self, uuid: _Optional[str] = ..., value: _Optional[_Union[_types_pb2.FeatureValue, _Mapping]] = ..., feature_descriptor: _Optional[_Union[_types_pb2.FeatureDescriptor, _Mapping]] = ...) -> None: ...

class FeatureDescriptorRequest(_message.Message):
    __slots__ = ("uuid", "selector")
    UUID_FIELD_NUMBER: _ClassVar[int]
    SELECTOR_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    selector: str
    def __init__(self, uuid: _Optional[str] = ..., selector: _Optional[str] = ...) -> None: ...

class FeatureDescriptorResponse(_message.Message):
    __slots__ = ("uuid", "feature_descriptor")
    UUID_FIELD_NUMBER: _ClassVar[int]
    FEATURE_DESCRIPTOR_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    feature_descriptor: _types_pb2.FeatureDescriptor
    def __init__(self, uuid: _Optional[str] = ..., feature_descriptor: _Optional[_Union[_types_pb2.FeatureDescriptor, _Mapping]] = ...) -> None: ...

class SetRequest(_message.Message):
    __slots__ = ("uuid", "selector", "keys", "value", "timestamp")
    class KeysEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    UUID_FIELD_NUMBER: _ClassVar[int]
    SELECTOR_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    selector: str
    keys: _containers.ScalarMap[str, str]
    value: _types_pb2.Value
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, uuid: _Optional[str] = ..., selector: _Optional[str] = ..., keys: _Optional[_Mapping[str, str]] = ..., value: _Optional[_Union[_types_pb2.Value, _Mapping]] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class SetResponse(_message.Message):
    __slots__ = ("uuid", "timestamp")
    UUID_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, uuid: _Optional[str] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AppendRequest(_message.Message):
    __slots__ = ("uuid", "fqn", "keys", "value", "timestamp")
    class KeysEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    UUID_FIELD_NUMBER: _ClassVar[int]
    FQN_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    fqn: str
    keys: _containers.ScalarMap[str, str]
    value: _types_pb2.Scalar
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, uuid: _Optional[str] = ..., fqn: _Optional[str] = ..., keys: _Optional[_Mapping[str, str]] = ..., value: _Optional[_Union[_types_pb2.Scalar, _Mapping]] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AppendResponse(_message.Message):
    __slots__ = ("uuid", "timestamp")
    UUID_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, uuid: _Optional[str] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class IncrRequest(_message.Message):
    __slots__ = ("uuid", "fqn", "keys", "value", "timestamp")
    class KeysEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    UUID_FIELD_NUMBER: _ClassVar[int]
    FQN_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    fqn: str
    keys: _containers.ScalarMap[str, str]
    value: _types_pb2.Scalar
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, uuid: _Optional[str] = ..., fqn: _Optional[str] = ..., keys: _Optional[_Mapping[str, str]] = ..., value: _Optional[_Union[_types_pb2.Scalar, _Mapping]] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class IncrResponse(_message.Message):
    __slots__ = ("uuid", "timestamp")
    UUID_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, uuid: _Optional[str] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class UpdateRequest(_message.Message):
    __slots__ = ("uuid", "selector", "keys", "value", "timestamp")
    class KeysEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    UUID_FIELD_NUMBER: _ClassVar[int]
    SELECTOR_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    selector: str
    keys: _containers.ScalarMap[str, str]
    value: _types_pb2.Value
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, uuid: _Optional[str] = ..., selector: _Optional[str] = ..., keys: _Optional[_Mapping[str, str]] = ..., value: _Optional[_Union[_types_pb2.Value, _Mapping]] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class UpdateResponse(_message.Message):
    __slots__ = ("uuid", "timestamp")
    UUID_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    uuid: str
    timestamp: _timestamp_pb2.Timestamp
    def __init__(self, uuid: _Optional[str] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

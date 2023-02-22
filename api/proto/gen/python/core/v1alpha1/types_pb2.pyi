from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Primitive(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    PRIMITIVE_UNSPECIFIED: _ClassVar[Primitive]
    PRIMITIVE_STRING: _ClassVar[Primitive]
    PRIMITIVE_INTEGER: _ClassVar[Primitive]
    PRIMITIVE_FLOAT: _ClassVar[Primitive]
    PRIMITIVE_BOOL: _ClassVar[Primitive]
    PRIMITIVE_TIMESTAMP: _ClassVar[Primitive]
    PRIMITIVE_STRING_LIST: _ClassVar[Primitive]
    PRIMITIVE_INTEGER_LIST: _ClassVar[Primitive]
    PRIMITIVE_FLOAT_LIST: _ClassVar[Primitive]
    PRIMITIVE_BOOL_LIST: _ClassVar[Primitive]
    PRIMITIVE_TIMESTAMP_LIST: _ClassVar[Primitive]

class AggrFn(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    AGGR_FN_UNSPECIFIED: _ClassVar[AggrFn]
    AGGR_FN_SUM: _ClassVar[AggrFn]
    AGGR_FN_AVG: _ClassVar[AggrFn]
    AGGR_FN_MAX: _ClassVar[AggrFn]
    AGGR_FN_MIN: _ClassVar[AggrFn]
    AGGR_FN_COUNT: _ClassVar[AggrFn]
PRIMITIVE_UNSPECIFIED: Primitive
PRIMITIVE_STRING: Primitive
PRIMITIVE_INTEGER: Primitive
PRIMITIVE_FLOAT: Primitive
PRIMITIVE_BOOL: Primitive
PRIMITIVE_TIMESTAMP: Primitive
PRIMITIVE_STRING_LIST: Primitive
PRIMITIVE_INTEGER_LIST: Primitive
PRIMITIVE_FLOAT_LIST: Primitive
PRIMITIVE_BOOL_LIST: Primitive
PRIMITIVE_TIMESTAMP_LIST: Primitive
AGGR_FN_UNSPECIFIED: AggrFn
AGGR_FN_SUM: AggrFn
AGGR_FN_AVG: AggrFn
AGGR_FN_MAX: AggrFn
AGGR_FN_MIN: AggrFn
AGGR_FN_COUNT: AggrFn

class Scalar(_message.Message):
    __slots__ = ["string_value", "int_value", "float_value", "bool_value", "timestamp_value"]
    STRING_VALUE_FIELD_NUMBER: _ClassVar[int]
    INT_VALUE_FIELD_NUMBER: _ClassVar[int]
    FLOAT_VALUE_FIELD_NUMBER: _ClassVar[int]
    BOOL_VALUE_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_VALUE_FIELD_NUMBER: _ClassVar[int]
    string_value: str
    int_value: int
    float_value: float
    bool_value: bool
    timestamp_value: _timestamp_pb2.Timestamp
    def __init__(self, string_value: _Optional[str] = ..., int_value: _Optional[int] = ..., float_value: _Optional[float] = ..., bool_value: bool = ..., timestamp_value: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class List(_message.Message):
    __slots__ = ["values"]
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedCompositeFieldContainer[Scalar]
    def __init__(self, values: _Optional[_Iterable[_Union[Scalar, _Mapping]]] = ...) -> None: ...

class Value(_message.Message):
    __slots__ = ["scalar_value", "list_value"]
    SCALAR_VALUE_FIELD_NUMBER: _ClassVar[int]
    LIST_VALUE_FIELD_NUMBER: _ClassVar[int]
    scalar_value: Scalar
    list_value: List
    def __init__(self, scalar_value: _Optional[_Union[Scalar, _Mapping]] = ..., list_value: _Optional[_Union[List, _Mapping]] = ...) -> None: ...

class ObjectReference(_message.Message):
    __slots__ = ["name", "namespace"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    name: str
    namespace: str
    def __init__(self, name: _Optional[str] = ..., namespace: _Optional[str] = ...) -> None: ...

class KeepPrevious(_message.Message):
    __slots__ = ["versions", "over"]
    VERSIONS_FIELD_NUMBER: _ClassVar[int]
    OVER_FIELD_NUMBER: _ClassVar[int]
    versions: int
    over: _duration_pb2.Duration
    def __init__(self, versions: _Optional[int] = ..., over: _Optional[_Union[_duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class FeatureDescriptor(_message.Message):
    __slots__ = ["fqn", "primitive", "aggr", "freshness", "staleness", "timeout", "keep_previous", "keys", "builder", "data_source", "runtime_env"]
    FQN_FIELD_NUMBER: _ClassVar[int]
    PRIMITIVE_FIELD_NUMBER: _ClassVar[int]
    AGGR_FIELD_NUMBER: _ClassVar[int]
    FRESHNESS_FIELD_NUMBER: _ClassVar[int]
    STALENESS_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_FIELD_NUMBER: _ClassVar[int]
    KEEP_PREVIOUS_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    BUILDER_FIELD_NUMBER: _ClassVar[int]
    DATA_SOURCE_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_ENV_FIELD_NUMBER: _ClassVar[int]
    fqn: str
    primitive: Primitive
    aggr: _containers.RepeatedScalarFieldContainer[AggrFn]
    freshness: _duration_pb2.Duration
    staleness: _duration_pb2.Duration
    timeout: _duration_pb2.Duration
    keep_previous: KeepPrevious
    keys: _containers.RepeatedScalarFieldContainer[str]
    builder: str
    data_source: str
    runtime_env: str
    def __init__(self, fqn: _Optional[str] = ..., primitive: _Optional[_Union[Primitive, str]] = ..., aggr: _Optional[_Iterable[_Union[AggrFn, str]]] = ..., freshness: _Optional[_Union[_duration_pb2.Duration, _Mapping]] = ..., staleness: _Optional[_Union[_duration_pb2.Duration, _Mapping]] = ..., timeout: _Optional[_Union[_duration_pb2.Duration, _Mapping]] = ..., keep_previous: _Optional[_Union[KeepPrevious, _Mapping]] = ..., keys: _Optional[_Iterable[str]] = ..., builder: _Optional[str] = ..., data_source: _Optional[str] = ..., runtime_env: _Optional[str] = ...) -> None: ...

class FeatureValue(_message.Message):
    __slots__ = ["fqn", "keys", "value", "timestamp", "fresh"]
    class KeysEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    FQN_FIELD_NUMBER: _ClassVar[int]
    KEYS_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    FRESH_FIELD_NUMBER: _ClassVar[int]
    fqn: str
    keys: _containers.ScalarMap[str, str]
    value: Value
    timestamp: _timestamp_pb2.Timestamp
    fresh: bool
    def __init__(self, fqn: _Optional[str] = ..., keys: _Optional[_Mapping[str, str]] = ..., value: _Optional[_Union[Value, _Mapping]] = ..., timestamp: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., fresh: bool = ...) -> None: ...

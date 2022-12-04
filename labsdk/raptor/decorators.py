# Copyright (c) 2022 RaptorML authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import inspect
import sys
import types
from datetime import timedelta
from typing import Union, List, Dict, Optional

from pandas import DataFrame
from pydantic import create_model_from_typeddict
from typing_extensions import TypedDict

from . import replay, local_state, config, durpy
from .program import Program
from .program import normalize_fqn
from .types import FeatureSpec, AggrSpec, AggregationFunction, FeatureSetSpec, \
    validate_timedelta, Primitive, DataSourceSpec

if sys.version_info >= (3, 8):
    from typing import TypedDict as typing_TypedDict
else:
    typing_TypedDict = type(None)


def _wrap_decorator_err(f):
    def wrap(*args, **kwargs):
        try:
            return f(*args, **kwargs)
        except Exception as e:
            back_frame = e.__traceback__.tb_frame.f_back
            tb = types.TracebackType(tb_next=None,
                                     tb_frame=back_frame,
                                     tb_lasti=back_frame.f_lasti,
                                     tb_lineno=back_frame.f_lineno)
            raise Exception(f"in {args[0].__name__}: {str(e)}").with_traceback(tb)

    return wrap


@_wrap_decorator_err
def _opts(func, options: dict):
    if hasattr(func, "raptor_spec"):
        raise Exception("option decorators must be before the registration decorator")
    if not hasattr(func, "__raptor_options"):
        func.__raptor_options = {}

    for k, v in options.items():
        func.__raptor_options[k] = v
    return func


# ** Shared **
def namespace(namespace: str):
    """
    Register a namespace for the Feature Definition.
    :param namespace: namespace name
    """

    def decorator(func):
        return _opts(func, {"namespace": namespace})

    return decorator


def runtime(
    packages: Optional[List[str]],  # list of PIP installable packages
    env_name: Optional[str],  # the Raptor virtual environment name
):
    """
    Register the runtime environment for the Feature Definition.
    :param packages:
    :param env_name:
    :return:
    """

    def decorator(func):
        return _opts(func, {"runtime": {
            "packages": packages,
            "env_name": env_name,
        }})

    return decorator


def freshness(
    target: Union[str, timedelta],
    invalid_after: Optional[Union[str, timedelta]] = None,  # defaults to == target
    latency_sla: Optional[Union[str, timedelta]] = timedelta(seconds=2),  # defaults to 2 seconds
):
    """
    Set the freshness, staleness, and latency of a feature or model.
    Must be used in conjunction with a feature or model decorator.
    Is placed AFTER the @model or @feature decorator.

    :param latency_sla: the maximum time allowed for the feature to be computed.
    :param invalid_after:
    :type target: object
    """
    if invalid_after is None:
        invalid_after = target

    def decorator(func):
        return _opts(func, {"freshness": {
            "target": target,
            "invalid_after": invalid_after,
            "latency_sla": latency_sla,
        }})

    return decorator


def labels(labels: Dict[str, str]):
    """
    Register labels for the Feature Definition.
    :param labels: a dictionary of tags.
    """

    def decorator(func):
        return _opts(func, {"labels": labels})

    return decorator


# ** Data Source **

# TODO
def data_source(
    training_data: DataFrame,  # training data

    keys: Optional[Union[str, List[str]]] = None,
    name: Optional[str] = None,  # inferred from class name
    timestamp: Optional[str] = None,  # what column has the timestamp
    production_data: Optional[object] = None,  # production data
):
    """
    Register a DataSource for the FeatureDefinition.
    """

    options = {}

    if isinstance(keys, str):
        keys = [keys]

    @_wrap_decorator_err
    def decorator(cls: TypedDict):
        if isinstance(cls, typing_TypedDict):
            raise Exception("You should use typing_extensions.TypedDict instead of typing.TypedDict")
        elif type(cls) != type(TypedDict):
            raise Exception("data_source decorator must be used on a class that extends typing_extensions.TypedDict")

        spec = DataSourceSpec()
        spec.keys = keys
        spec.description = cls.__doc__
        spec.name = name
        if name is None:
            spec.name = cls.__name__
        spec.timestamp = timestamp
        spec._local_df = training_data

        if hasattr(cls, "__raptor_options"):
            for k, v in cls.__raptor_options.items():
                options[k] = v

        # convert cls to json schema
        spec.schema = create_model_from_typeddict(cls).schema()

        # register
        cls.raptor_spec = spec
        cls.manifest = spec.manifest
        cls.export = spec.manifest
        local_state.register_spec(spec)

        if hasattr(cls, "__raptor_options"):
            del cls.__raptor_options

        return cls

    return decorator


# ** Feature **

def aggregation(
    function: Union[AggregationFunction, List[AggregationFunction], str, List[str]],
    over: Union[str, timedelta, None],
    granularity: Union[str, timedelta, None],
):
    """
    Register aggregations for the Feature Definition.
    :param over: the time period over which to aggregate.
    :param granularity: the granularity of the aggregation (this is overriding the freshness).
    :param function: a list of :func:`AggrFn`
    """

    if isinstance(function, str):
        function = [AggregationFunction.parse(function)]

    if not isinstance(function, List):
        function = [function]

    for i, f in function:
        if isinstance(f, str):
            function[i] = AggregationFunction.parse(f)

    if isinstance(over, str):
        over = durpy.from_str(over)
    if isinstance(granularity, str):
        granularity = durpy.from_str(granularity)

    validate_timedelta(over)
    validate_timedelta(granularity)

    def decorator(func):
        for f in function:
            if f == AggregationFunction.Unknown:
                raise Exception("Unknown aggr function")
        return _opts(func, {"aggr": AggrSpec(function, over, granularity)})

    return decorator


def feature(
    keys: Union[str, List[str]],
    name: Optional[str] = None,  # set to function name if not provided
    data_source: Optional[Union[str, object]] = None,  # set to None for headless
    keep_previous: Optional[int] = 0,  # Set to keep `versions` previous versions of this feature's value.
):
    """
    Register a Feature Definition within the LabSDK.

    A feature definition is a Python handler function that process a calculation request and calculates

    :param keys: a list of indexing keys, indicated the owner of the feature value.
    :param name: the name of the feature. If not provided, the function name will be used.
    :param data_source: the (fully qualified) name of the DataSource.
    :param keep_previous: Set to keep `versions` previous versions of this feature's value.

    :return: a registered Feature Definition
    """
    options = {}

    if isinstance(keys, str):
        keys = [keys]

    @_wrap_decorator_err
    def decorator(func):
        spec = FeatureSpec()
        spec.keys = keys
        spec.description = func.__doc__
        spec.name = name
        if name is None:
            spec.name = func.__name__

        if hasattr(func, "__raptor_options"):
            for k, v in func.__raptor_options.items():
                options[k] = v

        spec.data_source = data_source

        # append annotations
        if "labels" in options:
            spec.labels = options['labels']

        if "namespace" in options:
            spec.namespace = options['namespace']

        if "aggr" in options:
            spec.freshness = options['aggr'].granularity
            spec.staleness = options['aggr'].over

        if "freshness" in options:
            spec.freshness = options['freshness']["target"]
            spec.staleness = options['freshness']["invalid_after"]
            spec.timeout = options['freshness']["latency_sla"]

        if spec.freshness is None or spec.staleness is None:
            raise Exception("You must specify freshness or aggregation for a feature")

        if "runtime" in options:
            spec.builder.runtime = options['runtime']["env_name"]
            spec.builder.packages = options['runtime']["packages"]

        # parse the program
        def fqn_resolver(obj: str) -> str:
            frame = inspect.currentframe().f_back.f_back

            feat: Union[FeatureSpec, None] = None
            if obj in frame.f_globals:
                if hasattr(frame.f_globals[obj], "raptor_spec"):
                    feat = frame.f_globals[obj].raptor_spec
            elif obj in frame.f_locals:
                if hasattr(frame.f_locals[obj], "raptor_spec"):
                    feat = frame.f_locals[obj].raptor_spec
            if feat is None:
                raise Exception(f"Cannot resolve {obj} to an FQN")

            if feat.aggr is not None:
                raise Exception("You must specify a FQN with AggrFn(i.e. `namespace.name+sum`) for aggregated features")

            return feat.fqn()

        spec.program = Program(func, fqn_resolver)
        spec.primitive = Primitive.parse(spec.program.primitive)

        # aggr parsing should be after program parsing
        if "aggr" in options:
            for f in options['aggr'].funcs:
                if not f.supports(spec.primitive):
                    raise Exception(
                        f"{func.__name__} aggr function {f} not supported for primitive {spec.primitive}")
            spec.aggr = options['aggr']

        # register
        func.raptor_spec = spec
        func.replay = replay.new_replay(spec)
        func.manifest = spec.manifest
        func.export = spec.manifest
        local_state.register_spec(spec)

        if hasattr(func, "__raptor_options"):
            del func.__raptor_options

        return func

    return decorator


# ** Model **

def feature_set(register=False, options=None):
    """
    Register a FeatureSet Definition.

    A feature set definition in the LabSDK is constituted by a function that returns a list of features.
        You can specify a feature in the list using its FQN or via a variable that hold's the feature definition.
        When specifying a feature that have aggregations, you **must** specify the feature using its FQN.

    :Example:
        @feature_set(register=True)
        def my_feature_set():
            return [my_feature, "my_other_feature.default[sum]"]


    :param register: if True, the feature set will be registered in the LabSDK, and you'll be able to export
        it's manifest.
    :param options:
    :return:
    """
    if options is None:
        options = {}

    @_wrap_decorator_err
    def decorator(func):
        if inspect.signature(func) != inspect.signature(lambda: []):
            raise Exception(f"{func.__name__} have an invalid signature for a FeatureSet definition")

        fts = []
        for f in func():
            if type(f) is str:
                local_state.feature_spec_by_fqn(f)
                fts.append(normalize_fqn(f, config.default_namespace))
            if callable(f):
                ft = local_state.feature_spec_by_src_name(f.__name__)
                if ft is None:
                    raise Exception("Feature not found")
                if ft.aggr is not None:
                    raise Exception(
                        "You must specify a FQN with AggrFn(i.e. `namespace.name+sum`) for aggregated features")
                fts.append(ft.fqn())

        if hasattr(func, "__raptor_options"):
            for k, v in func.__raptor_options.items:
                options[k] = v

        spec = FeatureSetSpec()
        spec.name = func.__name__
        spec.description = func.__doc__
        spec.features = fts

        if "key_feature" in options:
            spec.key_feature = options["key_feature"]

        if "namespace" in options:
            spec.namespace = options["namespace"]

        if "timeout" not in options:
            options["timeout"] = "5s"

        spec.timeout = options["timeout"]
        spec.name = func.__name__
        spec.description = func.__doc__

        func.raptor_spec = spec
        func.historical_get = replay.new_historical_get(spec)
        func.manifest = spec.manifest
        func.export = spec.manifest
        local_state.register_spec(spec)

        if hasattr(func, "__raptor_options"):
            del func.__raptor_options

        if register:
            local_state.register_spec(spec)
        return func

    return decorator

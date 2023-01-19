# -*- coding: utf-8 -*-
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
from typing import Union, List, Dict, Optional, Callable

from pandas import DataFrame
from pydantic import create_model_from_typeddict
from typing_extensions import TypedDict

from . import local_state, config, durpy, replay
from .model import TrainingContext
from .program import Program
from .program import normalize_selector
from .types import FeatureSpec, AggrSpec, AggregationFunction, ModelSpec, \
    validate_timedelta, Primitive, DataSourceSpec, ModelFramework, ModelServer

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
            raise Exception(f'in {args[0].__name__}: {str(e)}').with_traceback(tb)

    return wrap


@_wrap_decorator_err
def _opts(func, options: dict):
    if hasattr(func, 'raptor_spec'):
        raise Exception('option decorators must be before the registration decorator')
    if not hasattr(func, '__raptor_options'):
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
        return _opts(func, {'namespace': namespace})

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
        return _opts(func, {'runtime': {
            'packages': packages,
            'env_name': env_name,
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
        return _opts(func, {'freshness': {
            'target': target,
            'invalid_after': invalid_after,
            'latency_sla': latency_sla,
        }})

    return decorator


def labels(labels: Dict[str, str]):
    """
    Register labels for the Feature Definition.
    :param labels: a dictionary of tags.
    """

    def decorator(func):
        return _opts(func, {'labels': labels})

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
        if type(cls) == type(typing_TypedDict):
            raise Exception('You should use typing_extensions.TypedDict instead of typing.TypedDict')
        elif type(cls) != type(TypedDict):
            raise Exception('data_source decorator must be used on a class that extends typing_extensions.TypedDict')

        spec = DataSourceSpec()
        spec.keys = keys
        spec.description = cls.__doc__
        spec.name = name
        if name is None:
            spec.name = cls.__name__
        spec.timestamp = timestamp
        spec._local_df = training_data

        if hasattr(cls, '__raptor_options'):
            for k, v in cls.__raptor_options.items():
                options[k] = v

        if 'labels' in options:
            spec.labels = options['labels']

        if 'namespace' in options:
            spec.namespace = options['namespace']

        # convert cls to json schema
        spec.schema = create_model_from_typeddict(cls).schema()

        # register
        cls.raptor_spec = spec
        cls.manifest = spec.manifest
        cls.export = spec.manifest
        local_state.register_spec(spec)

        if hasattr(cls, '__raptor_options'):
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

    for i, f in enumerate(function):
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
                raise Exception('Unknown aggr function')
        return _opts(func, {'aggr': AggrSpec(function, over, granularity)})

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

    if not isinstance(keys, List):
        keys = [keys]

    @_wrap_decorator_err
    def decorator(func):
        spec = FeatureSpec()
        spec.keys = keys
        spec.description = func.__doc__
        spec.name = name
        if name is None:
            spec.name = func.__name__

        if hasattr(func, '__raptor_options'):
            for k, v in func.__raptor_options.items():
                options[k] = v

        spec.data_source = data_source

        # append annotations
        if 'labels' in options:
            spec.labels = options['labels']

        if 'namespace' in options:
            spec.namespace = options['namespace']

        if 'aggr' in options:
            spec.freshness = options['aggr'].granularity
            spec.staleness = options['aggr'].over

        if 'freshness' in options:
            spec.freshness = options['freshness']['target']
            spec.staleness = options['freshness']['invalid_after']
            spec.timeout = options['freshness']['latency_sla']

        if spec.freshness is None or spec.staleness is None:
            raise Exception('You must specify freshness or aggregation for a feature')

        if 'runtime' in options:
            spec.builder.runtime = options['runtime']['env_name']
            spec.builder.packages = options['runtime']['packages']

        # parse the program
        def feature_obj_resolver(obj: str) -> str:
            """
            Resolve a feature object to its fully qualified name.
            :param obj:  the object name as defined in the global scope of the feature function.
            :return: the fully qualified name of the object.
            """
            frame = inspect.currentframe().f_back.f_back

            feat: Union[FeatureSpec, None] = None
            if obj in frame.f_globals:
                if hasattr(frame.f_globals[obj], 'raptor_spec'):
                    feat = frame.f_globals[obj].raptor_spec
            elif obj in frame.f_locals:
                if hasattr(frame.f_locals[obj], 'raptor_spec'):
                    feat = frame.f_locals[obj].raptor_spec
            if feat is None:
                raise Exception(f'Cannot resolve {obj} to an FQN')

            if feat.aggr is not None:
                raise Exception('You must specify a Feature Selector with AggrFn(i.e. `namespace.name+sum`) for '
                                'aggregated features')

            return feat.fqn()

        spec.program = Program(func, feature_obj_resolver)
        spec.primitive = Primitive.parse(spec.program.primitive)

        # aggr parsing should be after program parsing
        if 'aggr' in options:
            for f in options['aggr'].funcs:
                if not f.supports(spec.primitive):
                    raise Exception(
                        f'{func.__name__} aggr function {f} not supported for primitive {spec.primitive}')
            spec.aggr = options['aggr']

        # register
        func.raptor_spec = spec
        func.replay = replay.new_replay(spec)
        func.manifest = spec.manifest
        func.export = spec.manifest
        local_state.register_spec(spec)

        if hasattr(func, '__raptor_options'):
            del func.__raptor_options

        return func

    return decorator


# ** Model **

def model(
    keys: Union[str, List[str]],  # required
    input_features: Union[str, List[str], Callable, List[Callable]],  # required
    input_labels: Union[str, List[str], Callable, List[Callable]],
    model_framework: Union[ModelFramework, str],  # first is MVP
    model_server: Optional[Union[ModelServer, str]] = None,  # first is MVP
    key_feature: Optional[Union[str, Callable]] = None,  # optional
    prediction_output_schema: Optional[TypedDict] = None,
    name: Optional[str] = None,  # set to function name if not provided
):
    """
    Register a Model Definition within the LabSDK.
    """
    options = {}

    if not isinstance(keys, List):
        keys = [keys]

    if model_server is not None:
        model_server = ModelServer.parse(model_server)

    model_framework = ModelFramework.parse(model_framework)

    def normalize_features(features: Union[str, List[str], Callable, List[Callable]]) -> List[str]:
        if not isinstance(features, List):
            features = [features]

        fts = []
        for f in features:
            if type(f) is str:
                local_state.feature_spec_by_selector(f)
                fts.append(normalize_selector(f, config.default_namespace))
            elif isinstance(f, Callable):
                if not hasattr(f, 'raptor_spec'):
                    raise Exception(f'{f.__name__} is not a registered feature')
                if f.raptor_spec.aggr is not None:
                    raise Exception(f'{f.__name__} is an aggregated feature. You must specify a Feature Selector.')
                fts.append(f.raptor_spec.fqn())
            elif isinstance(f, FeatureSpec):
                fts.append(f.fqn())
        return fts

    input_features = normalize_features(input_features)
    input_labels = normalize_features(input_labels)
    key_feature = normalize_features(key_feature)[0] if key_feature is not None else None

    @_wrap_decorator_err
    def decorator(func):
        if len(inspect.signature(func).parameters) != 1:
            raise Exception(f'{func.__name__} must have a single parameter of type ModelContext')

        spec = ModelSpec()
        spec.keys = keys
        spec.description = func.__doc__
        spec.name = name
        if name is None:
            spec.name = func.__name__
            if spec.name.endswith('_model'):
                spec.name = spec.name[:-6]
            if spec.name.endswith('_trainer'):
                spec.name = spec.name[:-8]
            if spec.name.endswith('_train'):
                spec.name = spec.name[:-6]
            if spec.name.startswith('train_'):
                spec.name = spec.name[6:]

        if hasattr(func, '__raptor_options'):
            for k, v in func.__raptor_options.items():
                options[k] = v

        spec.features = input_features
        spec.label_features = input_labels
        spec.key_feature = key_feature
        spec.model_framework = model_framework
        spec.model_server = model_server

        if 'namespace' in options:
            spec.namespace = options['namespace']
        if 'labels' in options:
            spec.labels = options['labels']

        if 'freshness' in options:
            spec.freshness = options['freshness']['target']
            spec.staleness = options['freshness']['invalid_after']
            spec.timeout = options['freshness']['latency_sla']

        if spec.freshness is None or spec.staleness is None:
            raise Exception('You must specify freshness')

        local_state.register_spec(spec)

        if hasattr(func, '__raptor_options'):
            del func.__raptor_options

        features_and_labels = replay.new_historical_get(spec)

        def train():
            for f in (input_features + input_labels + ([key_feature] if key_feature is not None else [])):
                s = local_state.feature_spec_by_selector(f)
                replay.new_replay(s)()
            model = func(TrainingContext(
                keys=spec.keys,
                input_labels=spec.label_features,
                input_features=spec.features,
                data_getter=features_and_labels,
            ))
            spec.model_framework.save(model, spec)
            spec._trained_model = model
            spec._training_code = inspect.getsource(func)
            return model

        train.raptor_spec = spec
        train.features_and_labels = features_and_labels
        train.manifest = spec.manifest
        train.export = spec.manifest
        train.keys = spec.keys
        train.input_labels = spec.label_features
        train.input_features = spec.features

        return train

    return decorator

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
from warnings import warn

from pandas import DataFrame
from pydantic import create_model_from_typeddict
from typing_extensions import TypedDict

from . import local_state, config, durpy, replay
from .program import Program
from .program import normalize_selector
from .types import FeatureSpec, AggrSpec, AggregationFunction, Primitive, DataSourceSpec, ModelFramework, ModelServer, \
    KeepPreviousSpec, ModelImpl
from .types.dsrc_config_stubs.protocol import SourceProductionConfig
from .types.dsrc_config_stubs.rest import RestConfig

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

    :type target: timedelta or str of the form '2h 3m 4s'
    :param target: the target freshness of the feature or model.
    :type invalid_after: timedelta or str of the form '2h 3m 4s'
    :param invalid_after: the time after which the feature or model is considered stale.
    :type latency_sla: timedelta or str of the form '2h 3m 4s'
    :param latency_sla: the maximum time allowed for the feature to be computed.
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

def data_source(
    training_data: DataFrame,  # training data
    keys: Optional[Union[str, List[str]]] = None,
    name: Optional[str] = None,  # inferred from class name
    timestamp: Optional[str] = None,  # what column has the timestamp
    production_config: Optional[SourceProductionConfig] = None,  # production stub configuration
):
    """
    Register a DataSource for the FeatureDefinition.
    :param training_data: DataFrame of training data. This should reflect the schema of the data source in production.
    :param keys: list of columns that are keys.
    :param name: name of the data source. Defaults to the class name.
    :param timestamp: name of the timestamp column. If not specified, the timestamp is inferred from the training data.
    :type production_config: this is a stub for the production configuration. It is not used in training, but is helpful
            for making sense of the source, the production behavior, and a preparation for the production deployment.
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

        nonlocal name
        if name is None:
            name = cls.__name__

        spec = DataSourceSpec(name=name, description=cls.__doc__, keys=keys, timestamp=timestamp,
                              production_config=production_config)
        spec.local_df = training_data

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
    :type function: AggregationFunction or List[AggregationFunction] or str or List[str]
    :param function: a list of :func:`AggrFn`.
    :type over: str or timedelta in the form '2h 3m 4s'
    :param over: the time period over which to aggregate.
    :type granularity: str or timedelta in the form '2h 3m 4s'
    :param granularity: the granularity of the aggregation (this is overriding the freshness).
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

    def decorator(func):
        for fn in function:
            if fn == AggregationFunction.Unknown:
                raise Exception('Unknown aggr function')
        return _opts(func, {'aggr': AggrSpec(function, over, granularity)})

    return decorator


def keep_previous(versions: int, over: Union[str, timedelta]):
    """
    Keep previous versions of the feature.
    :type versions: int
    :param versions: the number of versions to keep (excluding the current value).
    :type over: str or timedelta in the form '2h 3m 4s'
    :param over: the maximum time period to keep a previous values in the history since the last update. You can specify
                    `0` to keep the value until the next update.
    version is computed.
    """

    if isinstance(over, str):
        over = durpy.from_str(over)

    def decorator(func):
        return _opts(func, {'keep_previous': KeepPreviousSpec(versions, over)})

    return decorator


def feature(
    keys: Union[str, List[str]],
    name: Optional[str] = None,  # set to function name if not provided
    data_source: Optional[Union[str, object]] = None,  # set to None for sourceless
    sourceless_markers_df: Optional[DataFrame] = None,  # timestamp and keys markers for training sourceless features
):
    """
    Register a Feature Definition within the LabSDK.

    A feature definition is a Python handler function that process a calculation request and calculates

    :param keys: a list of indexing keys, indicated the owner of the feature value.
    :param name: the name of the feature. If not provided, the function name will be used.
    :param data_source: the (fully qualified) name of the DataSource.
    :param sourceless_markers_df: a DataFrame with the timestamp and keys markers for training sourceless features. It
            a timestamp column, and a column for each key.

    :return: a registered Feature Definition
    """
    options = {}

    if not isinstance(keys, List):
        keys = [keys]

    @_wrap_decorator_err
    def decorator(func):
        nonlocal name
        if name is None:
            name = func.__name__

        spec = FeatureSpec(name=name, description=func.__doc__, keys=keys)

        if hasattr(func, '__raptor_options'):
            for k, v in func.__raptor_options.items():
                options[k] = v

        spec.data_source = data_source
        spec.sourceless_df = sourceless_markers_df

        # append annotations
        if 'labels' in options:
            spec.labels = options['labels']

        if 'namespace' in options:
            spec.namespace = options['namespace']

        if 'aggr' in options:
            spec.freshness = options['aggr'].granularity
            spec.staleness = options['aggr'].over
            if data_source is not None and isinstance(data_source, RestConfig):
                warn('Beware: aggregations for REST might not behave as you expect. '
                     'Read the documentation for more info.')

        if 'freshness' in options:
            spec.freshness = options['freshness']['target']
            spec.staleness = options['freshness']['invalid_after']
            spec.timeout = options['freshness']['latency_sla']

        if 'keep_previous' in options:
            spec.keep_previous = options['keep_previous']

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

        nonlocal name
        if name is None:
            name = func.__name__
        if name.endswith('_model'):
            name = name[:-6]
        if name.endswith('_trainer'):
            name = name[:-8]
        if name.endswith('_train'):
            name = name[:-6]
        if name.startswith('train_'):
            name = name[6:]

        spec = ModelImpl(name=name, description=func.__doc__, keys=keys, model_framework=model_framework,
                         model_server=model_server)

        spec.features = input_features
        spec.label_features = input_labels
        spec.key_feature = key_feature

        if hasattr(func, '__raptor_options'):
            for k, v in func.__raptor_options.items():
                options[k] = v

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

        if 'runtime' in options:
            spec.runtime.runtime = options['runtime']['env_name']
            spec.runtime.packages = options['runtime']['packages']

        local_state.register_spec(spec)

        if hasattr(func, '__raptor_options'):
            del func.__raptor_options

        spec.training_function = func

        def trainer():
            return spec.train()

        trainer.train = trainer
        trainer.raptor_spec = spec
        trainer.features_and_labels = spec.features_and_labels
        trainer.manifest = spec.manifest
        trainer.export = spec.export
        trainer.keys = spec.keys
        trainer.input_labels = spec.label_features
        trainer.input_features = spec.features

        return trainer

    return decorator

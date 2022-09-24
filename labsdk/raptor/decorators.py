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
import types as pytypes
from . import replay, local_state, stub
from .types import FeatureSpec, AggrSpec, ResourceReference, AggrFn, PyExpProgram, BuilderSpec, \
    FeatureSetSpec, normalize_fqn, PyExpException


def _wrap_decorator_err(f):
    def wrap(*args, **kwargs):
        try:
            return f(*args, **kwargs)
        except PyExpException as e:
            raise e
        except Exception as e:
            back_frame = e.__traceback__.tb_frame.f_back
            tb = pytypes.TracebackType(tb_next=None,
                                       tb_frame=back_frame,
                                       tb_lasti=back_frame.f_lasti,
                                       tb_lineno=back_frame.f_lineno)
            raise Exception(f"in {args[0].__name__}: {str(e)}").with_traceback(tb)

    return wrap


@_wrap_decorator_err
def _opts(func, options: dict):
    if hasattr(func, "raptor_spec"):
        raise Exception("option decorators must be before the register decorator")
    if not hasattr(func, "__raptor_options"):
        func.__raptor_options = {}

    for k, v in options.items():
        func.__raptor_options[k] = v
    return func


def aggr(funcs: [AggrFn], granularity=None):
    """
    Register aggregations for the Feature Definition.
    :param granularity: the granularity of the aggregation (this is overriding the freshness).
    :param funcs: a list of :func:`AggrFn`
    """

    def decorator(func):
        for f in funcs:
            if f == AggrFn.Unknown:
                raise Exception("Unknown aggr function")
        return _opts(func, {"aggr": AggrSpec(funcs, granularity)})

    return decorator


def connector(name: str, namespace: str = None):
    """
    Register a DataConnector for the FeatureDefinition.
    :param name: the name of the DataConnector.
    :param namespace: the namespace of the DataConnector.
    """

    def decorator(func):
        return _opts(func, {"connector": ResourceReference(name, namespace)})

    return decorator


def namespace(namespace: str):
    """
    Register a namespace for the Feature Definition.
    :param namespace: namespace name
    """

    def decorator(func):
        return _opts(func, {"namespace": namespace})

    return decorator


def builder(kind: str, options=None):
    """
    Register a builder for the Feature Definition.
    :param kind: the kind of the builder.
    :param options: options for the builder.
    :return:
    """

    if options is None:
        options = {}

    def decorator(func):
        return _opts(func, {"builder": BuilderSpec(kind, options)})

    return decorator


def _func_match_feature_signature(func):
    def _stub_feature(**req):
        pass

    def _stub_feature_with_req(**req: stub.RaptorRequest):
        pass

    sig = inspect.signature(func)
    return sig == inspect.signature(_stub_feature) or sig == inspect.signature(_stub_feature_with_req)


def register(primitive, staleness: str, freshness: str = '', options=None):
    """
    Register a Feature Definition within the LabSDK.

    A feature definition is a PyExp handler function that process a calculation request and calculates
    a feature value for it::
        :param RaptorRequest **kwargs: a request bag dictionary(:class:`RaptorRequest`)
        :return: a tuple of (value, timestamp, entity_id) where:
            - value: the feature value
            - timestamp: the timestamp of the value - when None, it uses the request timestamp.
            - entity_id: the entity id of the value - when None, it uses the request entity_id.
                If the request entity_id is None, it's required to specify an entity_id

    :Example:
        @register(primitive="my_primitive", freshness="1h", staleness="1h")
        def my_feature(**req):
            return "Hello "+ req["entity_id"]

    :param primitive: the primitive type of the feature.
    :param staleness: the staleness of the feature.
    :param freshness: the freshness of the feature.
    :param options: optional options for the feature.
    :return: a registered Feature Definition
    """
    if options is None:
        options = {}

    @_wrap_decorator_err
    def decorator(func):
        if not _func_match_feature_signature(func):
            raise Exception(f"{func.__name__} have an invalid signature for a Feature definition")

        spec = FeatureSpec()
        spec.freshness = freshness
        spec.staleness = staleness
        spec.primitive = primitive
        spec.name = func.__name__
        spec.description = func.__doc__

        if hasattr(func, "__raptor_options"):
            for k, v in func.__raptor_options.items():
                options[k] = v

        # append annotations
        if "builder" in options:
            spec.builder = options['builder']

        if "namespace" in options:
            spec.namespace = options['namespace']

        if "connector" in options:
            spec.connector = options['connector']
        if "aggr" in options:
            for f in options['aggr'].funcs:
                if not f.supports(spec.primitive):
                    raise Exception(
                        f"{func.__name__} aggr function {f} not supported for primitive {spec.primitive}")
            spec.aggr = options['aggr']

        if freshness == '' and (spec.aggr is None or spec.aggr.granularity is None):
            raise Exception(f"{func.__name__} must have a freshness or an aggregation with granularity")
        if staleness == '':
            raise Exception(f"{func.__name__} must have a staleness")

        # add source coded (decorators stripped)
        spec.program = PyExpProgram(func, spec.fqn())

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
                local_state.spec_by_fqn(f)
                fts.append(normalize_fqn(f))
            if callable(f):
                ft = local_state.spec_by_src_name(f.__name__)
                if ft is None:
                    raise Exception("Feature not found")
                if ft.aggr is not None:
                    raise Exception(
                        "You must specify a FQN with AggrFn(i.e. `name.namespace[sum]`) for aggregated features")
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


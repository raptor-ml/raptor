# Copyright (c) 2022 Raptor.
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

import datetime
import inspect
import re
import types as pytypes

from . import types, replay, local_state, stub
from .pyexp import pyexp


def aggr(funcs: [types.AggrFn]):
    """
    Register aggregations for the Feature Definition.
    :param funcs: a list of :func:`types.AggrFn`
    """

    def decorator(func):
        for f in funcs:
            if f == types.AggrFn.Unknown:
                raise Exception("Unknown aggr function")
        func.aggr = funcs
        return func

    return decorator


def connector(name: str, namespace: str = ""):
    """
    Register a DataConnector for the FeatureDefinition.
    :param fqn: DataConnector Fully Qualified Name(*FQN*)
    """

    def decorator(func):
        func.connector = {"name": name, "namespace": namespace}
        return func

    return decorator


def namespace(namespace: str):
    """
    Register a namespace for the Feature Definition.
    When the namespace is not specified, it's assumed to be "default".
    :param namespace: namespace name
    """

    def decorator(func):
        func.namespace = namespace
        return func

    return decorator


def builder(kind: str, options=None):
    """
    Register a builder for the Feature Definition.
    :param kind: the kind of the builder.
    :param options: options for the builder.
    :return:
    """

    def decorator(func):
        func.builder = {"kind": kind, "options": options}
        return func

    return decorator


def _stub_feature(**req):
    pass


def _stub_feature_with_req(**req: stub.RaptorRequest):
    pass


def register(primitive, freshness: str, staleness: str, options=None):
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
    :param freshness: the freshness of the feature.
    :param staleness: the staleness of the feature.
    :param options: optional options for the feature.
    :return: a registered Feature Definition
    """
    if options is None:
        options = {}

    @wrap_decorator_err
    def decorator(func):
        if inspect.signature(func) != inspect.signature(_stub_feature) and inspect.signature(func) != inspect.signature(
                _stub_feature_with_req):
            raise Exception(f"{func.__name__} have an invalid signature for a Feature definition")

        options["freshness"] = freshness
        options["staleness"] = staleness

        if 'name' not in options and (func.__name__ == '<lambda>' or func.__name__ != 'handler'):
            options['name'] = func.__name__
        elif 'name' in options and options['name'] != 'handler':
            raise Exception("Function name is required")

        if 'desc' not in options and func.__doc__ is not None:
            options['desc'] = func.__doc__

        # verify primitive
        p = primitive
        if p == 'int' or p == int:
            p = 'int'
        elif p == 'float' or p == float:
            p = 'float'
        elif p == 'timestamp' or p == datetime:
            p = 'timestamp'
        elif p == 'str' or p == str:
            p = 'string'
        elif p == '[]str' or p == [str]:
            p = '[]str'
        elif p == '[]int' or p == [int]:
            p = '[]int'
        elif p == '[]float' or p == [float]:
            p = '[]float'
        elif p == '[]timestamp' or p == [datetime]:
            p = '[]timestamp'
        elif p == 'headless':
            p = 'headless'
        else:
            raise Exception("Primitive type not supported")
        options['primitive'] = p

        # append annotations
        if hasattr(func, "builder"):
            options["builder"] = func.builder

        if hasattr(func, "namespace"):
            options["namespace"] = func.namespace
        if "namespace" not in options:
            options["namespace"] = "default"

        if hasattr(func, "connector"):
            options["connector"] = func.connector
        if hasattr(func, "aggr"):
            for f in func.aggr:
                if not f.supports(options["primitive"]):
                    raise Exception(
                        f"{func.__name__} aggr function {f} not supported for primitive {options['primitive']}")
            options["aggr"] = func.aggr

        # build the spec
        # add source coded (decorators stripped)
        src = types.PyExpProgram(func)
        fqn = f"{options['name']}.{options['namespace']}"
        spec = {"kind": "feature", "options": options, "src": src, "src_name": func.__name__, "fqn": fqn}
        func.raptor_spec = spec

        # try to compile the feature
        pyexp.New(spec["src"].code, fqn)

        # register
        func.replay = replay.new_replay(spec)
        func.manifest = lambda: __feature_manifest(spec)
        local_state.register_spec(spec)

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

    @wrap_decorator_err
    def decorator(func):
        if inspect.signature(func) != inspect.signature(lambda: []):
            raise Exception(f"{func.__name__} have an invalid signature for a FeatureSet definition")

        fts = []
        for f in func():
            if type(f) is str:
                local_state.spec_by_fqn(f)
                fts.append(f)
            if callable(f):
                ft = local_state.spec_by_src_name(f.__name__)
                if ft is None:
                    raise Exception("Feature not found")
                if "aggr" in ft["options"]:
                    raise Exception("You must specify a FQN with AggrFn for aggregated features")
                fts.append(ft["fqn"])

        if "key_feature" not in options:
            options["key_feature"] = fts[0]

        if hasattr(func, "namespace"):
            options["namespace"] = func.namespace
        if "namespace" not in options:
            options["namespace"] = "default"

        if "timeout" not in options:
            options["timeout"] = "5s"

        if "name" not in options:
            options["name"] = func.__name__

        if "desc" not in options and func.__doc__ is not None:
            options["desc"] = func.__doc__

        fqn = f"{options['name']}.{options['namespace']}"
        spec = {"kind": "feature_set", "options": options, "src": fts, "src_name": func.__name__, "fqn": fqn}
        func.raptor_spec = spec
        func.historical_get = replay.new_historical_get(spec)
        func.manifest = lambda: __feature_set_manifest(spec)
        if register:
            local_state.register_spec(spec)
        return func

    return decorator


def _k8s_name(name):
    name = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    name = re.sub('__([A-Z])', r'_\1', name)
    name = re.sub('([a-z0-9])([A-Z])', r'\1_\2', name)
    return name.replace("_", "-").lower()


def __feature_manifest(f):
    def _fmt(val, field=None):
        if val is None:
            return "~"
        elif field in val:
            return val[field]
        return "~"

    t = f"""apiVersion: k8s.raptor.ml/v1alpha1
kind: Feature
metadata:
  name: {_k8s_name(f['options']['name'])}
  namespace: {f['options']['namespace']}"""
    if _fmt(f['options'], 'desc') != "~":
        t += f"""\n  annotations:\n    a8r.io/description: "{_fmt(f['options'], 'desc')}"""""
    t += f"""\nspec:
  primitive: {_fmt(f['options'], 'primitive')}
  freshness: {_fmt(f['options'], 'freshness')}
  staleness: {_fmt(f['options'], 'staleness')}"""
    if 'timeout' in f['options']:
        t += f"\n  timeout: {_fmt(f['options'], 'timeout')}"
    if 'connector' in f['options']:
        t += f"\n  connector: \n    name: {_fmt(f['options']['connector'], 'name')}"
        if 'namespace' in f['options']['connector'] and f['options']['connector']['namespace'] != "":
            t += f"\n    namespace: {_fmt(f['options']['connector'], 'namespace')}"
    t += "\n  builder:"
    if 'builder' in f['options']:
        if 'kind' in f['options']['builder']:
            t += f"\n    kind: {_fmt(f['options']['builder'], 'kind')}"
        if 'options' in ['builder']['options']:
            for k, v in f['options']['builder']['options']:
                t += f"    {k}: {_fmt(v)}\n"
    if 'aggr' in f['options']:
        t += "\n    aggr:"
        for a in f['options']['aggr']:
            t += "\n      - " + a.value
    t += "\n    pyexp: |"
    for line in f['src'].code.split('\n'):
        t += "\n      " + line
    t += "\n"
    return t


def __feature_set_manifest(f):
    nl = "\n"
    ret = f"""apiVersion: k8s.raptor.ml/v1alpha1
kind: FeatureSet
metadata:
  name: {_k8s_name(f["options"]["name"])}
  namespace: {f["options"]["namespace"]}
spec:
  timeout: {f["options"]["timeout"]}
  keyFeature: {f["options"]["key_feature"]}
  features:\n"""
    for ft in f["src"]:
        ret += f"    - {ft}\n"
    return ret


def manifests(save_to_tmp=False):
    """
    manifests will create a list of registered Raptor manifests ready to install for your kubernetes cluster

    If save_to_tmp is True, it will save the manifests to a temporary file and return the path to the file.
    Otherwise, it will print the manifests.
    """
    mfts = []
    for m in local_state.spec_registry:
        if m["kind"] == "feature":
            mfts.append(__feature_manifest(m))
        elif m["kind"] == "feature_set":
            mfts.append(__feature_set_manifest(m))
        else:
            raise Exception("Invalid manifest")

    if len(mfts) == 0:
        return ""

    ret = '---\n'.join(mfts)
    if save_to_tmp:
        import tempfile
        f = tempfile.NamedTemporaryFile(mode='w+t', delete=False)
        f.write(ret)
        file_name = f.name
        f.close()
        return file_name
    else:
        return ret


def wrap_decorator_err(f):
    def wrap(*args, **kwargs):
        try:
            return f(*args, **kwargs)
        except RuntimeError as e:
            raise types.WrapException(e, args[0].raptor_spec)
        except Exception as e:
            back_frame = e.__traceback__.tb_frame.f_back
            tb = pytypes.TracebackType(tb_next=None,
                                       tb_frame=back_frame,
                                       tb_lasti=back_frame.f_lasti,
                                       tb_lineno=back_frame.f_lineno)
            raise Exception(f"in {args[0].__name__}: {str(e)}").with_traceback(tb)

    return wrap

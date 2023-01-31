# -*- coding: utf-8 -*-
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

# !! Notice !!
# This is a shared file between the runtime and the core.
# Do not import anything from the runtime or core here.

import ast
import builtins
import datetime as dt_pkg
import hashlib
import importlib
import re
from datetime import datetime
from inspect import getsource, getsourcefile, getsourcelines
from pydoc import locate
from typing import List, Dict, Callable, Union, Tuple, Optional

from redbaron import RedBaron, DefNode

selector_regex = re.compile(
    r'^((?P<namespace>([a0-z9]+[a0-z9_]*[a0-z9]+){1,256})\.)?(?P<name>([a0-z9]+[a0-z9_]*[a0-z9]+){1,256})(\+(?P<aggrFn>([a-z]+_*[a-z]+)))?(@-(?P<version>([0-9]+)))?(\[(?P<encoding>([a-z]+_*[a-z]+))])?$',
    re.IGNORECASE | re.DOTALL)

primitive = Union[str, int, float, bool, datetime, List[str], List[int], List[float], List[bool], List[datetime], None]


def normalize_fqn(fqn, default_namespace='default'):
    matches = selector_regex.match(fqn)
    if matches is None:
        raise Exception(f'Invalid selector: {fqn}')

    namespace = matches.group('namespace')
    name = matches.group('name')

    if namespace is None:
        namespace = default_namespace

    return f'{namespace}.{name}'


def normalize_selector(selector, default_namespace='default'):
    matches = selector_regex.match(selector)
    if matches is None:
        raise Exception(f'Invalid selector: {selector}')
    namespace = matches.group('namespace')
    name = matches.group('name')
    aggr_fn = matches.group('aggrFn')
    version = matches.group('version')
    encoding = matches.group('encoding')

    if namespace is None:
        namespace = default_namespace

    extra = ''
    if aggr_fn is not None and aggr_fn != '':
        extra += f'+{aggr_fn}'
    if version is not None and version != '':
        extra += f'@-{version}'
    if encoding is not None and encoding != '':
        extra += f'[{encoding}]'

    return f'{namespace}.{name}{extra}'


_blocked_builtins = [
    'compile', 'eval', 'exec', 'open', 'input', 'file', 'dir', 'quit', 'exit', 'InterruptedError', 'SystemExit',
    'PermissionError', 'ProcessLookupError', 'NotADirectoryError', 'FileNotFoundError', 'BrokenPipeError',
    'ConnectionError', 'ConnectionAbortedError', 'ConnectionRefusedError', 'ConnectionResetError', 'BlockingIOError',
    'ChildProcessError', 'FileExistsError'
]

safe_builtins = {}
for name in builtins.__dict__:
    if name not in _blocked_builtins:
        safe_builtins[name] = builtins.__dict__[name]

_blocked_dataset_packages = [
    'pandas',
]
_blocked_modeling_packages = [
    'sklearn', 'tensorflow', 'xgboost', 'lightgbm', 'catboost', 'torch', 'torchvision', 'torchaudio', 'keras', 'fastai',
    'pytorch_lightning', 'onnx', 'onnxruntime', 'onnxruntime_tools', 'onnxruntime_training', 'onnxruntime_gpu_tensorrt',
    'scipy', 'matplotlib', 'scikit-learn', 'scikit-image', 'onnxruntime-gpu', 'Theano-PyMC', 'datascience',
    'imbalanced', 'opencv-contrib-python', 'opencv-python', 'sklearn-pandas', 'librosa', 'h5py', 'PyWavelets',
    'pmdarima', 'sktime', 'statsmodels', 'pytorch-lightning', 'seaborn', 'transformers', 'theano', 'nltk', 'hdf5',
]
_blocked_io_packages = [
    'os', 'sys', 'subprocess', 'shutil', 'http', 'urllib', 'socket', 'multiprocessing', 'threading',
    'subprocess', 'os', 'shutil', 'winreg', 'requests', 'threading', 'tempfile', 'urllib3', 'urllib2', 'urllib',
    'asyncio', 'io', 'gevent', 'grequests', 'aiohttp', 'uplink', 'httpx', 'builtins',
]


def secure_importer(name, globals=None, locals=None, fromlist=(), level=0):
    if name in (_blocked_dataset_packages + _blocked_modeling_packages + _blocked_io_packages):
        raise builtins.ImportError("module '%s' is restricted." % name)

    return importlib.__import__(name, globals, locals, fromlist, level)


safe_builtins['__import__'] = secure_importer

_side_effect_ctx_functions = ['get_feature', 'get_prediction']


class SideEffect:
    def __init__(self, kind: str, args: Dict[str, str], conditional: bool):
        self.kind = kind
        self.args = args
        self.conditional = conditional


class Context:
    """
    Context of the feature request.
    :param str fqn: Fully Qualified Name of the feature.
    :param Dict[str,str] keys: the keys(identifiers) we request the value for.
    :param datetime timestamp: the timestamp when the request for this feature was made by the user
    """

    fqn: str
    keys: Dict[str, str]
    timestamp: datetime

    def __init__(self,
                 fqn: str,
                 keys: Dict[str, str],
                 timestamp: datetime,
                 feature_getter: Callable[
                     [str, Dict[str, str], datetime], Tuple[Optional[primitive], Optional[datetime]]],
                 prediction_getter: Callable[[str, Dict[str, str], datetime], Tuple[primitive, datetime]],
                 ):

        parsed = selector_regex.match(fqn)
        if parsed is None:
            raise Exception(f'Invalid FQN. Got: {fqn}')
        if parsed.group('namespace') is None:
            raise Exception(f'FQN with a namespace is mandatory when defining a context. Got: {fqn}')

        self.namespace = parsed.group('namespace')
        self.fqn = fqn
        self.keys = keys
        self.timestamp = timestamp
        self.__feature_getter = feature_getter
        self.__prediction_getter = prediction_getter

    def get_feature(self, selector: str, keys: Dict[str, str] = None) -> [primitive, datetime]:
        """
        Get feature value for a dependant feature.

        Behind the scenes, the LabSDK will return you the value for the requested Feature Selector and keys
        **at the appropriate** timestamp of the request. That means that we'll use the request's timestamp when replying
        features. Cool right? 😎

        :param str selector: Feature Selector of the feature, including aggregation function if exists.
        :param str keys: the keys(identifiers) we request the value for.
        :return: a tuple of (value, timestamp)
        """

        if keys is None:
            keys = self.keys
        selector = normalize_selector(selector, self.namespace)
        return self.__feature_getter(selector, keys, self.timestamp)

    def get_prediction(self, selector: str, keys: Dict[str, str] = None) -> Tuple[any, datetime]:
        """
        Get the predicted value from a model.

        Behind the scenes, the LabSDK will call the model server and deliver back the prediction from the requested
        Model's FQN and keys **at the appropriate** timestamp of the request. That means that we'll use the
        request's timestamp when replying features. Cool right? 😎

        :param str selector: Fully Qualified Name of the model.
        :param str keys: the keys(identifiers) we request the value for. By default, the keys of the current context are
         used.
        :return: a tuple of (prediction, timestamp)
        """

        if keys is None:
            keys = self.keys
        selector = normalize_selector(selector, self.namespace)
        return self.__prediction_getter(selector, keys, self.timestamp)


class Program:
    name: str
    handler: callable
    globals: dict = {}
    locals: dict = {}
    primitive: [str, int, float, datetime, List[str], List[int], List[float], List[datetime], None]
    side_effects: List[SideEffect] = []

    code: str
    checksum: bytes

    src_file: Optional[str] = None
    src_line: Optional[int] = None

    def __init__(self, code, feature_obj_resolver: Callable[[str], str] = None):
        if isinstance(code, Callable):
            self.src_file = getsourcefile(code)
            self.src_line = getsourcelines(code)[1]
            code = getsource(code)
        m = hashlib.sha256()
        m.update(code.encode('utf-8'))
        self.checksum = m.digest()

        root_node = RedBaron(code)
        if len(root_node) != 1:
            raise SyntaxError('PythonRuntime supports one function definition')
        node = root_node[0]
        if not isinstance(node, DefNode):
            raise SyntaxError('PythonRuntime only supports function definition')

        # We must remove any decorators left in the function definition
        if len(node.decorators) > 0:
            if self.src_line is not None:
                self.src_line += len(node.decorators)
            node.decorators = []

        if len(node.arguments) != 2:
            raise SyntaxError('Feature function requires exactly 2 arguments: (this_row, context)')

        # Remove arguments annotations
        if node.arguments:
            for arg in node.arguments:
                arg.annotation = ''

        ctx_arg = node.arguments[1].name.value

        for imp in (node.find_all('import') + node.find_all('fromimport')):
            iname = imp.name.value
            if iname in _blocked_dataset_packages:
                raise SyntaxError(
                    '🛑 You should not use dataset packages(e.g. Pandas) in a Feature function. '
                    "Remember: use the reactive mindset - \"work on a row level, but you always have a state\"")

            if iname in _blocked_modeling_packages:
                raise SyntaxError("🛑 You shouldn't use modeling packages here. Feature functions are made for "
                                  'calculating the data toward a dataset for the model.')
            if iname in _blocked_io_packages:
                raise SyntaxError('🛑 Importing i/o packages are restricted for Feature functions.')

        for at in node.find_all('call'):
            if at.parent.name.value == node.name:
                raise SyntaxError('🛑 Recursion is restricted for Feature function')
            if at.parent.name.value == ctx_arg:
                method = at.parent.value[1].value
                if method in _side_effect_ctx_functions:
                    args = {}
                    for arg in at.find_all('call_argument'):
                        if (arg.index_on_parent == 0 or (arg.target is not None and arg.target.value == 'selector')) \
                            and arg.value.type != 'string':
                            if feature_obj_resolver is None:
                                raise SyntaxError('🛑 You must provide a Feature Selector for this Feature function')
                            args['selector'] = feature_obj_resolver(arg.value.value)

                        if arg.target is not None:
                            args[arg.target.value] = arg.value.value
                        args[arg.index_on_parent] = arg.value.value

                        self.side_effects.append(
                            SideEffect(kind=method, args=args, conditional=at.parent_find('if') is not None))

        self.name = node.name
        rav = node.return_annotation.name.value
        rav = 'datetime.datetime' if rav == 'datetime' else rav

        if rav == 'List':
            itm = node.return_annotation.value.getitem.value.value
            if not isinstance(itm, str):
                itm = itm.name.value
            scalar = locate('datetime.datetime' if itm == 'datetime' else itm)
            self.primitive = List[scalar]
        else:
            self.primitive = locate(rav)

        self.code = node.dumps().strip()

        compiled = None
        if self.src_file is not None and self.src_line is not None:
            parsed = ast.parse(self.code, filename=self.src_file)
            ast.increment_lineno(parsed, self.src_line - 1)
            compiled = compile(parsed, filename=self.src_file, mode='exec')
        else:
            compiled = compile(self.code, f'<{self.name}>', 'exec')

        glob, loc = {'__builtins__': safe_builtins, 'datetime': dt_pkg, 'List': List}, {}
        exec(compiled, glob, loc)

        self.handler = loc[self.name]
        self.globals = glob
        self.locals = loc

    def call(self, data: Dict[str, primitive], context: Context):
        return self.handler(data, context)

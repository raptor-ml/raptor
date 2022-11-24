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

import builtins
import importlib
from datetime import datetime
from pydoc import locate

from redbaron import RedBaron, DefNode
from typing import List, Dict, Callable

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
    "sklearn", "tensorflow", "xgboost", "lightgbm", "catboost", "torch", "torchvision", "torchaudio", "keras", "fastai",
    "pytorch_lightning", "onnx", "onnxruntime", "onnxruntime_tools", "onnxruntime_training", "onnxruntime_gpu_tensorrt",
    "scipy", "matplotlib", "scikit-learn", "scikit-image", "onnxruntime-gpu", "Theano-PyMC", "datascience",
    "imbalanced", "opencv-contrib-python", "opencv-python", "sklearn-pandas", "librosa", "h5py", "PyWavelets",
    "pmdarima", "sktime", "statsmodels", "pytorch-lightning", "seaborn", "transformers", "theano", "nltk", "hdf5",
]
_blocked_io_packages = [
    "os", "sys", "subprocess", "shutil", "http", "urllib", "socket", "multiprocessing", "threading",
    "subprocess", "os", "shutil", "winreg", "requests", "threading", "tempfile", "urllib3", "urllib2", "urllib",
    "asyncio", "io", "gevent", "grequests", "aiohttp", "uplink", "httpx", "builtins",
]


def secure_importer(name, globals=None, locals=None, fromlist=(), level=0):
    if name in (_blocked_dataset_packages + _blocked_modeling_packages + _blocked_io_packages):
        raise builtins.ImportError("module '%s' is restricted." % name)

    return importlib.__import__(name, globals, locals, fromlist, level)


safe_builtins['__import__'] = secure_importer

_side_effect_ctx_functions = ["get_feature"]


class SideEffect:
    def __init__(self, kind: str, args: Dict[str, any], conditional: bool):
        self.kind = kind
        self.args = args
        self.conditional = conditional


class Context:
    def __init__(self,
                 fqn: str,
                 keys: Dict[str, str],
                 timestamp: datetime,
                 feature_getter: Callable[[str, Dict[str, str], datetime], any]
                 ):
        self.fqn = fqn
        self.keys = keys
        self.timestamp = timestamp
        self.__feature_getter = feature_getter

    def get_feature(self, fqn: str, keys: Dict[str, str] = None, timestamp: datetime = None) -> any:
        if keys is None:
            keys = self.keys
        if timestamp is None:
            timestamp = self.timestamp
        return self.__feature_getter(fqn, keys, timestamp)


class Program:
    name: str
    handler: callable
    globals: dict
    locals: dict
    primitive: [str, int, float, datetime, List[str], List[int], List[float], List[datetime], None]
    side_effects: List[SideEffect]

    def __init__(self, code, fqn_resolver: Callable[[object], str] = None):
        root_node = RedBaron(code)
        if len(root_node) != 1:
            raise SyntaxError("PythonRuntime supports one function definition")
        node = root_node[0]
        if not isinstance(node, DefNode):
            raise SyntaxError("PythonRuntime only supports function definition")

        # We must remove any decorators left in the function definition
        if len(node.decorators) > 0:
            node.decorators = []

        if len(node.arguments) != 2:
            raise SyntaxError("Feature function requires exactly 2 arguments: (this_row, context)")

        ctx_arg = node.arguments[1].name.value

        for imp in (node.find_all('import') + node.find_all('fromimport')):
            iname = imp.name.value
            if iname in _blocked_dataset_packages:
                raise SyntaxError(
                    "🛑 You should not use dataset packages(e.g. Pandas) in a Feature function. "
                    "Remember: use the reactive mindest - \"work on a row level, but you always have a state\"")

            if iname in _blocked_modeling_packages:
                raise SyntaxError("🛑 You shouldn't use modeling packages here. Feature functions are made for "
                                  "calculating the data toward a dataset for the model.")
            if iname in _blocked_io_packages:
                raise SyntaxError("🛑 Importing i/o packages are restricted for Feature functions.")

        for at in node.find_all("call"):
            if at.parent.name.value == node.name:
                raise SyntaxError("🛑 Recursion is restricted for Feature function")
            if at.parent.name.value == ctx_arg:
                method = at.parent.value[1].value
                if method in _side_effect_ctx_functions:
                    args = {}
                    for i, arg in at.find_all("call_argument"):
                        if i == 0 or (
                            arg.target is not None and arg.target.value == "fqn") and arg.value.value.type != "string":
                            if fqn_resolver is None:
                                raise SyntaxError("🛑 You must provide a FQN as a string for this Feature function")
                            args["fqn"] = fqn_resolver(arg.value.value)

                        if arg.target is not None:
                            args[arg.target.value] = arg.value.value
                        args[str(i)] = arg.value.value

                    self.side_effects.append(
                        SideEffect(kind=at.name.value, args=args, conditional=at.parent_find("if") is not None))

        self.name = node.name
        rav = node.return_annotation.value
        self.primitive = locate("datetime.datetime" if rav == "datetime" else rav)

        c = compile(root_node.dumps().strip(), f"<{self.name}>", "exec")
        glob, loc = {'__builtins__': safe_builtins}, {}
        exec(c, glob, loc)

        self.handler = loc[self.name]
        self.globals = glob
        self.locals = loc

    async def call(self, data: Dict[str, any], context: Context):
        return await self.handler(data, context)

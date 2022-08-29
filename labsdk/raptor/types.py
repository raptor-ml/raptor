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
import ast
import inspect
import re
import types
from enum import Enum

import astunparse as astunparse
import pandas as pd


class AggrFn(Enum):
    Unknown = 'unknown'
    Sum = 'sum'
    Avg = 'avg'
    Max = 'max'
    Min = 'min'
    Count = 'count'
    DistinctCount = 'distinct_count'
    ApproxDistinctCount = 'approx_distinct_count'

    def supports(self, type):
        if self == AggrFn.Unknown:
            return False
        if self in (AggrFn.Sum, AggrFn.Avg, AggrFn.Max, AggrFn.Min):
            return type in ('int', 'float')
        return True

    def apply(self, rgb: pd.core.window.RollingGroupby):
        if self == AggrFn.Sum:
            return rgb.sum()
        if self == AggrFn.Avg:
            return rgb.mean()
        if self == AggrFn.Max:
            return rgb.max()
        if self == AggrFn.Min:
            return rgb.min()
        if self == AggrFn.Count:
            return rgb.count()
        if self == AggrFn.DistinctCount or self == AggrFn.ApproxDistinctCount:
            return rgb.apply(lambda x: pd.Series(x).nunique())
        raise Exception(f"Unknown AggrFn {self}")


def WrapException(e: Exception, spec, *args, **kwargs):
    frame_str = re.match(r".*<pyexp>:([0-9]+):([0-9]+)?: (.*)", str(e).replace("\n", ""), flags=re.MULTILINE)
    if frame_str is None or not isinstance(spec["src"], PyExpProgram):
        return e
    else:
        err_str = re.match(r"in (.*)Error in ([aA0-zZ09_]+): (.*)", frame_str.group(3))
        if err_str is None:
            err_str = frame_str.group(3).strip()
        else:
            err_str = f"Error in {err_str.group(2)}: {err_str.group(3).strip()}"
        frame = spec["src"].frame
        loc = spec["src"].f_lineno + int(frame_str.group(1)) - 1
        tb = types.TracebackType(tb_next=None,
                                 tb_frame=frame,
                                 tb_lasti=e.__traceback__.tb_lasti,
                                 tb_lineno=loc)
        return PyExpException(
            f"on {spec['src_name']}:\n    {err_str}\n\n️Friendly tip: remember that PyExp is not python3 😬") \
            .with_traceback(tb)


class PyExpException(RuntimeError):
    def __int__(self, *args, **kwargs):
        Exception.__init__(*args, **kwargs)


class PyExpProgram:
    def __init__(self, func):
        # take a snapshot of the func frame
        self.frame = None
        try:
            raise Exception()
        except Exception as e:
            self.frame = e.__traceback__.tb_frame.f_back.f_back.f_back
            pass
        self.f_lineno = self.frame.f_lineno

        root_node = ast.parse(inspect.getsource(func))
        if len(root_node.body) != 1:
            raise RuntimeError("PyExpProgram in LabSDK only supports one function definition")
        node = root_node.body[0]
        if not isinstance(node, ast.FunctionDef):
            raise RuntimeError("PyExpProgram in LabSDK only supports function definition")
        if node.args.args:
            for arg in node.args.args:
                arg.annotation = None
        if node.args.kwarg:
            node.args.kwarg.annotation = None
        if len(node.decorator_list) > 0:
            self.f_lineno += len(node.decorator_list)
            node.decorator_list = []

        self.code = astunparse.unparse(node).strip()
        self.name = func.__name__

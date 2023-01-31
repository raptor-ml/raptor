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

from datetime import datetime
from typing import List

from . import EnumSpec


class Primitive(EnumSpec):
    String = 'string'
    Integer = 'int'
    Float = 'float'
    Boolean = 'bool'
    Timestamp = 'timestamp'
    StringList = '[]string'
    IntList = '[]int'
    FloatList = '[]float'
    BooleanList = '[]bool'
    TimestampList = '[]timestamp'

    def is_scalar(self):
        return self in (Primitive.String, Primitive.Integer, Primitive.Float, Primitive.Timestamp)

    @staticmethod
    def parse(p):
        if isinstance(p, Primitive):
            return p
        elif p == 'str' or p == str:
            return Primitive.String
        elif p == 'int' or p == int:
            return Primitive.Integer
        elif p == 'float' or p == float:
            return Primitive.Float
        elif p == 'bool' or p == bool:
            return Primitive.Boolean
        elif p == 'timestamp' or p == datetime:
            return Primitive.Timestamp
        elif p == '[]string' or p == List[str]:
            return Primitive.StringList
        elif p == '[]int' or p == List[int]:
            return Primitive.IntList
        elif p == '[]float' or p == List[float]:
            return Primitive.FloatList
        elif p == '[]bool' or p == List[bool]:
            return Primitive.BooleanList
        elif p == '[]timestamp' or p == List[datetime]:
            return Primitive.TimestampList
        else:
            raise Exception('Primitive type {p} not supported')

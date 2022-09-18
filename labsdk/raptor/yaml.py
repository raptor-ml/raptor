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

import datetime
from copy import deepcopy

import yaml

from . import durpy


# Raptor YAML Dumper
# - multiline strings
# - go duration
# - ignore None fields
# - no tags
# - no aliases
class RaptorDumper(yaml.SafeDumper):
    def ignore_aliases(self, data): return True

    def process_tag(self): return None


def _cleanup_none(d):
    for k, v in list(d.items()):
        if v is None:
            del d[k]
        elif isinstance(v, dict):
            _cleanup_none(v)
            if len(v) == 0:
                del d[k]
    return d


# ignore None fields
def _none_remove_representer(dumper, data):
    data = _cleanup_none(deepcopy(data))
    return dumper.represent_dict(data)


RaptorDumper.add_representer(dict, _none_remove_representer)


# enable multiline strings
def _str_representer(dumper, data):
    if len(data.splitlines()) > 1:  # check for multiline string
        return dumper.represent_scalar('tag:yaml.org,2002:str', data, style='|')
    return dumper.represent_scalar('tag:yaml.org,2002:str', data)


RaptorDumper.add_representer(str, _str_representer)


# represent timedelta with go duration
def _timedelta_representer(dumper, data):
    if data is None:
        return dumper.represent_none(data)
    return dumper.represent_scalar('tag:yaml.org,2002:str', durpy.to_str(data))


RaptorDumper.add_representer(datetime.timedelta, _timedelta_representer)

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

from typing import Optional

from typing_extensions import TypedDict, NotRequired


def get_feature(fqn: str, entity_id: str):
    """Get feature value for a dependant feature.

    Behind the scenes, the LabSDK will return you the value for the requested fqn and entity
    **at the appropriate** timestamp of the request. That means that we'll use the request's timestamp when replying
    features. Cool right? 😎

    :param str fqn: Fully Qualified Name of the feature, including aggregation function if exists.
    :param str entity_id: the entity identifier we request the value for.
    :return: a tuple of (value, timestamp)

    note::
        You can also use the alias :func:`f` to refer to this function.
    """
    pass


f = get_feature


def set_feature(fqn: str, entity_id: str, value, timestamp=None):
    """Set feature value.

    **Arguments**::

    :param str fqn: Fully Qualified Name of the feature.
    :param str entity_id: the entity identifier we would like to set the value for.
    :param value: the value we would like to set.
    :param Optional[time] timestamp: the timestamp we would like to set the value for. If not set, it'll use the request's timestamp.
    """
    pass


def update_feature(fqn: str, entity_id: str, value, timestamp=None):
    """Update feature value.

    **Arguments**::

    :param str fqn: Fully Qualified Name of the feature.
    :param str entity_id: the entity identifier we would like to update the value for.
    :param value: the value we would like to update.
    :param Optional[time] timestamp: the timestamp we would like to update the value for. If not set, it'll use the request's timestamp.
    """


def append_feature(fqn: str, entity_id: str, value, timestamp=None):
    """Append feature value (for features of list type).

    **Arguments**::

    :param str fqn: Fully Qualified Name of the feature.
    :param str entity_id: the entity identifier we would like to append the value to.
    :param value: the value we would like to append.
    :param Optional[time] timestamp: the timestamp we request the value for. If not set, it'll use the request's timestamp.
    """
    pass


def incr_feature(fqn: str, entity_id: str, by: float, timestamp=None):
    """Increment feature value.

    **Arguments**::

    :param str fqn: Fully Qualified Name of the feature.
    :param str entity_id: the entity identifier would like to increment the value to.
    :param float by: the amount to increment the value by. Tip: you can use negative values to decrement.
    :param Optional[time] timestamp: the timestamp we request the value for. If not set, it'll use the request's timestamp.
    """
    pass


class RaptorRequest(TypedDict):
    """RaptorRequest is the bag of arguments the every PyExp program receive while calculating feature value.

    It is a dictionary that consist with the following keys::

    :param str entity_id: the entity identifier that the feature is calculated for
        (i.e. user_id, item_id, etc.)
    :param object timestamp: the timestamp of the request. This is used to attach a timestamp to the feature value.
    :param object payload: the payload of the request (i.e. event data, etc.)
    :param dict headers: the headers of the request

    seealso:: https://raptor.ml/docs/reference/pyexp/handler-function#input-arguments-via-kwargs
    """

    entity_id: NotRequired[str]
    timestamp: object
    payload: NotRequired[object]
    headers: NotRequired[dict]

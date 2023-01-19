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


###
# this file represent the same bucket naming algorithm as in the Core: /api/windows.go
###

import string
from datetime import datetime, timedelta, timezone

microseconds = 1000000


# Truncate returns the result of rounding t down to a multiple of d (since the zero time).
#
# Truncate operates on the time as an absolute duration since the
# zero time; it does not operate on the presentation form of the
# time. Thus, Truncate(Hour) may return a time with a non-zero
# minute, depending on the time's Location.
def truncate(t: datetime, d: timedelta) -> datetime:
    if d <= timedelta(0):
        return t.replace(microsecond=0)

    td = t.replace(tzinfo=timezone.utc).timestamp() * microseconds
    return t - timedelta(microseconds=td % (d.total_seconds() * microseconds))


def int_to_base(x, base):
    digs = string.digits + string.ascii_letters

    if x < 0:
        sign = -1
    elif x == 0:
        return digs[0]
    else:
        sign = 1

    x *= sign
    digits = []

    while x:
        digits.append(digs[int(x % base)])
        x = x // base

    if sign < 0:
        digits.append('-')

    digits.reverse()
    return ''.join(digits)


# BucketName returns a bucket name for a given timestamp and a bucket size
def bucket_name(ts: datetime, bucket_size: timedelta) -> str:
    t = truncate(ts, bucket_size)
    ts = t.replace(tzinfo=timezone.utc).timestamp() * microseconds
    b = ts / (bucket_size.total_seconds() * microseconds)
    return int_to_base(b, 34)


# BucketTime returns the start time of a given bucket by its name
def bucket_time(name: str, bucket_size: timedelta) -> datetime:
    b = int(name, 34)
    return datetime.fromtimestamp(0, timezone.utc) + timedelta(
        microseconds=b * bucket_size.total_seconds() * microseconds)

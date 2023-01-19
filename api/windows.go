/*
Copyright (c) 2022 RaptorML authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// DeadGracePeriod is the *extra* time that the bucket should be kept alive on top of the feature's Staleness.
// Bucket TTL = staleness + DeadGracePeriod
const DeadGracePeriod = time.Minute * 10

// AggrFn is an aggregation function
type AggrFn int

const (
	AggrFnUnknown AggrFn = iota
	AggrFnSum
	AggrFnAvg
	AggrFnMax
	AggrFnMin
	AggrFnCount
)

func (w AggrFn) String() string {
	switch w {
	case AggrFnSum:
		return "sum"
	case AggrFnAvg:
		return "avg"
	case AggrFnMax:
		return "max"
	case AggrFnMin:
		return "min"
	case AggrFnCount:
		return "count"
	default:
		return "unknown"
	}
}

func StringsToAggrFns(fns []string) ([]AggrFn, error) {
	aggrFnsMap := make(map[AggrFn]bool)
	for _, fn := range fns {
		f := StringToAggrFn(fn)
		if f == AggrFnUnknown {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedAggrError, fn)
		}
		aggrFnsMap[f] = true
	}

	// Unique
	var aggrFns []AggrFn
	for f := range aggrFnsMap {
		aggrFns = append(aggrFns, f)
	}

	return aggrFns, nil
}
func StringToAggrFn(s string) AggrFn {
	switch strings.ToLower(s) {
	case "sum":
		return AggrFnSum
	case "avg", "mean":
		return AggrFnAvg
	case "min":
		return AggrFnMin
	case "max":
		return AggrFnMax
	case "count":
		return AggrFnCount
	default:
		return AggrFnUnknown
	}
}

// BucketName returns a bucket name for a given timestamp and a bucket size
func BucketName(ts time.Time, bucketSize time.Duration) string {
	b := ts.Truncate(bucketSize).UnixNano() / int64(bucketSize)
	return strconv.FormatInt(b, 34)
}

// BucketTime returns the start time of a given bucket by its name
func BucketTime(bucketName string, bucketSize time.Duration) time.Time {
	bucket, err := strconv.ParseInt(bucketName, 34, 64)
	if err != nil {
		panic(err)
	}
	return time.Unix(0, bucket*int64(bucketSize))
}

// BucketDeadTime returns the end time of a given bucket by its name
func BucketDeadTime(bucketName string, bucketSize, staleness time.Duration) time.Time {
	return BucketTime(bucketName, bucketSize).Add(staleness + DeadGracePeriod)
}

// AliveWindowBuckets returns a list of all the *valid* buckets up until now
func AliveWindowBuckets(staleness, bucketSize time.Duration) []string {
	numberOfBuckets := int(math.Ceil(float64(staleness) / float64(bucketSize)))
	now := time.Now()

	keys := make([]string, numberOfBuckets)
	for i := 0; i < numberOfBuckets; i++ {
		keys[i] = BucketName(now.Add(-bucketSize*time.Duration(i)), bucketSize)
	}
	return keys
}

// DeadWindowBuckets returns a list of bucket names of *dead* bucket (bucket that is outside the window) that should be available
func DeadWindowBuckets(staleness, bucketSize time.Duration) []string {
	ab := AliveWindowBuckets(staleness, bucketSize)
	deadTime := BucketTime(ab[len(ab)-1], bucketSize).Add(-1)
	numberOfBuckets := int(math.Ceil(float64(DeadGracePeriod) / float64(bucketSize)))

	keys := make([]string, numberOfBuckets)
	for i := 0; i < numberOfBuckets; i++ {
		keys[i] = BucketName(deadTime.Add(-bucketSize*time.Duration(i)), bucketSize)
	}
	return keys
}

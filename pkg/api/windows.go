/*
Copyright 2022 Natun.

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
	"github.com/natun-ai/natun/pkg/errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DeadGracePeriod is the *extra* time that the bucket should be kept alive on top of the feature's Staleness.
// Bucket TTL = staleness + DeadGracePeriod
const DeadGracePeriod = time.Minute * 8

//goland:noinspection RegExpRedundantEscape
var windowNameRegexp = regexp.MustCompile(`(i?)^([a0-z9\-\.]*)\[(sum|avg|min|max|count)\]$`)

func FQNToRealFQN(name string) (string, WindowFn) {
	matches := windowNameRegexp.FindStringSubmatch(name)
	if len(matches) < 3 {
		return name, WindowFnUnknown
	}
	return matches[1], StringToWindowFn(matches[2])
}

// WindowFn is an aggregation function
type WindowFn int

const (
	WindowFnUnknown WindowFn = iota
	WindowFnSum
	WindowFnAvg
	WindowFnMax
	WindowFnMin
	WindowFnCount
)

func (w WindowFn) String() string {
	switch w {
	case WindowFnSum:
		return "sum"
	case WindowFnAvg:
		return "avg"
	case WindowFnMax:
		return "max"
	case WindowFnMin:
		return "min"
	case WindowFnCount:
		return "count"
	default:
		return "unknown"
	}
}

func StringsToWindowFns(fns []string) ([]WindowFn, error) {
	windowFnsMap := make(map[WindowFn]bool)
	for _, fn := range fns {
		f := StringToWindowFn(fn)
		if f == WindowFnUnknown {
			return nil, fmt.Errorf("%w: %s", errors.ErrUnsupportedAggrError, fn)
		}
		windowFnsMap[f] = true
	}

	// Unique
	var windowFns []WindowFn
	for f := range windowFnsMap {
		windowFns = append(windowFns, f)
	}

	return windowFns, nil
}
func StringToWindowFn(s string) WindowFn {
	switch strings.ToLower(s) {
	case "sum":
		return WindowFnSum
	case "avg", "mean":
		return WindowFnAvg
	case "min":
		return WindowFnMin
	case "max":
		return WindowFnMax
	case "count":
		return WindowFnCount
	default:
		return WindowFnUnknown
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

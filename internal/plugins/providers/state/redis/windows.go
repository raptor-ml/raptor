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

package redis

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/raptor-ml/raptor/api"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MaxScanCount = 1000

func windowKey(FQN string, bucketName string, encodedKeys string) string {
	return fmt.Sprintf("%s/%s:%s", FQN, bucketName, encodedKeys)
}
func fromWindowKey(k string) (fqn string, bucketName string, encodedKeys string) {
	firstSep := strings.Index(k, "/")
	lastColon := strings.LastIndex(k, ":")
	return k[:firstSep], k[firstSep+1 : lastColon], k[lastColon+1:]
}

func (s *state) DeadWindowBuckets(ctx context.Context, fd api.FeatureDescriptor, ignore api.RawBuckets) (api.RawBuckets, error) {
	bucketNames := api.DeadWindowBuckets(fd.Staleness, fd.Freshness)

	wg := &sync.WaitGroup{}
	wg.Add(len(bucketNames))

	cRes := make(chan string)
	cErr := make(chan error)

	// find dead buckets
	for _, bucketName := range bucketNames {
		go func(bucketName string, wg *sync.WaitGroup, cRes chan string, cErr chan error) {
			defer wg.Done()

			itr := s.client.ScanType(ctx, 0, windowKey(fd.FQN, bucketName, "*"), MaxScanCount, "hash").Iterator()
			for itr.Next(ctx) {
				cRes <- itr.Val()
			}
			if itr.Err() != nil {
				cErr <- itr.Err()
			}
		}(bucketName, wg, cRes, cErr)
	}

	go func(wg *sync.WaitGroup) {
		wg.Wait()
		close(cRes)
	}(wg)

	var buckets []api.RawBucket
loop:
	for {
		select {
		case err := <-cErr:
			close(cRes)
			return nil, err
		case key, more := <-cRes:
			if !more {
				break loop
			}
			if ignoreKey(ignore, key) {
				continue
			}
			fqn, bucketName, encodedKeys := fromWindowKey(key)
			b := api.RawBucket{
				FQN:         fqn,
				Bucket:      bucketName,
				EncodedKeys: encodedKeys,
			}
			buckets = append(buckets, b)
		}
	}
	return s.windowBuckets(ctx, buckets)
}

func ignoreKey(ignore api.RawBuckets, key string) bool {
	for _, b := range ignore {
		if windowKey(b.FQN, b.Bucket, b.EncodedKeys) == key {
			return true
		}
	}
	return false
}

func (s *state) windowBuckets(ctx context.Context, buckets []api.RawBucket) (api.RawBuckets, error) {
	wg := &sync.WaitGroup{}
	wg.Add(len(buckets))

	cRes := make(chan api.RawBucket)
	cErr := make(chan error)
	for _, b := range buckets {
		go func(c chan api.RawBucket, wg *sync.WaitGroup, b api.RawBucket) {
			defer wg.Done()

			res, err := s.client.HGetAll(ctx, windowKey(b.FQN, b.Bucket, b.EncodedKeys)).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				cErr <- err
				return
			}
			if errors.Is(err, redis.Nil) || len(res) == 0 {
				return
			}

			rm := make(api.WindowResultMap)
			for k, v := range res {
				vv, err := strconv.ParseFloat(v, 64)
				if err != nil {
					cErr <- err
				}
				rm[api.StringToAggrFn(k)] = vv
			}
			c <- api.RawBucket{
				FQN:         b.FQN,
				Bucket:      b.Bucket,
				EncodedKeys: b.EncodedKeys,
				Data:        rm,
			}
		}(cRes, wg, b)
	}
	go func(group *sync.WaitGroup) {
		wg.Wait()
		close(cRes)
	}(wg)

	var res api.RawBuckets
	for {
		select {
		case err := <-cErr:
			close(cRes)
			return nil, err
		case v, more := <-cRes:
			if !more {
				return res, nil
			}
			res = append(res, v)
		}
	}
}
func (s *state) WindowBuckets(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, bucketNames []string) (api.RawBuckets, error) {
	var buckets api.RawBuckets
	encodedKeys, err := keys.Encode(fd)
	if err != nil {
		return nil, err
	}

	for _, b := range bucketNames {
		buckets = append(buckets, api.RawBucket{
			FQN:         fd.FQN,
			Bucket:      b,
			EncodedKeys: encodedKeys,
		})
	}
	buckets, err = s.windowBuckets(ctx, buckets)
	if err != nil {
		return nil, err
	}

	return buckets, nil
}

func (s *state) getWindow(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys) (*api.Value, error) {
	buckets, err := s.WindowBuckets(ctx, fd, keys, api.AliveWindowBuckets(fd.Staleness, fd.Freshness))
	if err != nil {
		return nil, err
	}

	var avg bool
	ret := make(api.WindowResultMap)
	for _, b := range buckets {
		for _, fn := range fd.Aggr {
			switch fn {
			case api.AggrFnCount, api.AggrFnSum:
				ret[fn] += b.Data[fn]
			case api.AggrFnMin:
				if _, ok := ret[fn]; !ok {
					ret[fn] = b.Data[fn]
				} else {
					ret[fn] = math.Min(ret[fn], b.Data[fn])
				}
			case api.AggrFnMax:
				if _, ok := ret[fn]; !ok {
					ret[fn] = b.Data[fn]
				} else {
					ret[fn] = math.Max(ret[fn], b.Data[fn])
				}
			case api.AggrFnAvg:
				avg = true
			}
		}
	}
	// Should be implicitly by the end
	if avg {
		ret[api.AggrFnAvg] = ret[api.AggrFnSum] / ret[api.AggrFnCount]
	}

	if len(ret) == 0 {
		return nil, nil
	}

	return &api.Value{
		Value:     ret,
		Timestamp: time.Now(),
		Fresh:     true,
	}, nil
}

func (s *state) WindowAdd(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, value any, ts time.Time) error {
	bucket := api.BucketName(ts, fd.Freshness)
	key, err := primitiveKey(fd, keys, 0)
	if err != nil {
		return err
	}

	var val float64
	switch v := value.(type) {
	case int:
		val = float64(v)
	case float64:
		val = v
	default:
		return fmt.Errorf("unsupported value type %T", value)
	}

	tx := s.client.TxPipeline()
	for _, fn := range fd.Aggr {
		switch fn {
		case api.AggrFnSum:
			tx.HIncrByFloat(ctx, key, "sum", val)
		case api.AggrFnCount:
			tx.HIncrBy(ctx, key, "count", 1)
		case api.AggrFnMin:
			luaHMin.Run(ctx, tx, []string{key, "min"}, val)
		case api.AggrFnMax:
			luaHMax.Run(ctx, tx, []string{key, "max"}, val)
		}
	}
	exp := api.BucketDeadTime(bucket, fd.Freshness, fd.Staleness)
	setTimestampExpireAt(ctx, tx, key, ts, exp)
	tx.PExpireAt(ctx, key, exp)

	_, err = tx.Exec(ctx)
	return err
}

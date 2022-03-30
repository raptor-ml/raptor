package redis

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/natun-ai/natun/pkg/api"
	"math"
	"strconv"
	"sync"
	"time"
)

type bucketData struct {
	bucket   string
	response *redis.StringStringMapCmd
}

func windowKey(FQN string, entityID string, bucketName string) string {
	return fmt.Sprintf("%s/%s:%s", FQN, bucketName, entityID)
}

func (s *state) WindowBuckets(ctx context.Context, md api.Metadata, entityID string, buckets []string) (api.RawBuckets, error) {
	c := make(chan bucketData)
	wg := &sync.WaitGroup{}
	wg.Add(len(buckets))
	for _, b := range buckets {
		go func(c chan bucketData, wg *sync.WaitGroup, b string) {
			defer wg.Done()
			c <- bucketData{
				bucket:   b,
				response: s.client.HGetAll(ctx, windowKey(md.FQN, entityID, b)),
			}
		}(c, wg, b)
	}
	go func(group *sync.WaitGroup) {
		wg.Wait()
		close(c)
	}(wg)

	ret := make(api.RawBuckets)
	for r := range c {
		if r.response.Err() != nil {
			return nil, r.response.Err()
		}
		for k, v := range r.response.Val() {
			vv, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, err
			}
			if _, ok := ret[r.bucket]; !ok {
				ret[r.bucket] = make(api.WindowResultMap)
			}
			ret[r.bucket][api.StringToWindowFn(k)] = vv
		}
	}
	return ret, nil
}

func (s *state) getWindow(ctx context.Context, md api.Metadata, entityID string) (*api.Value, error) {
	buckets := api.AliveWindowBuckets(md.Staleness, md.Freshness)
	windows, err := s.WindowBuckets(ctx, md, entityID, buckets)
	if err != nil {
		return nil, err
	}

	var avg bool
	ret := make(api.WindowResultMap)
	for _, w := range windows {
		for _, fn := range md.Aggr {
			switch fn {
			case api.WindowFnCount, api.WindowFnSum:
				ret[fn] += w[fn]
			case api.WindowFnMin:
				if _, ok := ret[fn]; !ok {
					ret[fn] = w[fn]
				} else {
					ret[fn] = math.Min(ret[fn], w[fn])
				}
			case api.WindowFnMax:
				if _, ok := ret[fn]; !ok {
					ret[fn] = w[fn]
				} else {
					ret[fn] = math.Max(ret[fn], w[fn])
				}
			case api.WindowFnAvg:
				avg = true
			}
		}
	}
	// Should be implicitly by the end
	if avg {
		ret[api.WindowFnAvg] = ret[api.WindowFnSum] / ret[api.WindowFnCount]
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

func (s *state) WindowAdd(ctx context.Context, md api.Metadata, entityID string, value any, ts time.Time) error {
	bucket := api.BucketName(ts, md.Freshness)
	key := windowKey(md.FQN, entityID, bucket)

	var val float64
	switch v := value.(type) {
	case int:
		val = float64(v)
	case float64:
		val = v
	}

	tx := s.client.TxPipeline()
	for _, fn := range md.Aggr {
		switch fn {
		case api.WindowFnSum:
			tx.HIncrByFloat(ctx, key, "sum", val)
		case api.WindowFnCount:
			tx.HIncrBy(ctx, key, "count", 1)
		case api.WindowFnMin:
			luaHMin.Run(ctx, tx, []string{key, "min"}, val)
		case api.WindowFnMax:
			luaHMax.Run(ctx, tx, []string{key, "max"}, val)
		}
	}
	exp := api.BucketTime(bucket, md.Freshness).Add(md.Staleness + api.DeadGracePeriod)
	setTimestampExpireAt(ctx, tx, key, ts, exp)
	tx.PExpireAt(ctx, key, exp)

	_, err := tx.Exec(ctx)
	return err
}

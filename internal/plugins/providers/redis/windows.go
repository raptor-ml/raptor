package redis

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/pkg/api"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MaxScanCount = 1000

func windowKey(FQN string, bucketName string, entityID string) string {
	return fmt.Sprintf("%s/%s:%s", FQN, bucketName, entityID)
}
func fromWindowKey(k string) (fqn string, bucketName string, entityID string) {
	firstSep := strings.Index(k, "/")
	lastColon := strings.LastIndex(k, ":")
	return fqn[:firstSep], fqn[firstSep+1 : lastColon], fqn[lastColon+1:]
}

func (s *state) DeadWindowBuckets(ctx context.Context, md api.Metadata, ignore api.RawBuckets) (api.RawBuckets, error) {
	bucketNames := api.DeadWindowBuckets(md.Staleness, md.Freshness)

	wg := &sync.WaitGroup{}
	wg.Add(len(bucketNames))

	cRes := make(chan string)
	cErr := make(chan error)

	// find dead buckets
	for _, bucketName := range bucketNames {
		go func(bucketName string, wg *sync.WaitGroup, cRes chan string, cErr chan error) {
			defer wg.Done()

			itr := s.client.ScanType(ctx, 0, windowKey(md.FQN, "*", ""), MaxScanCount, "hash").Iterator()
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
			fqn, bucketName, entityID := fromWindowKey(key)
			b := api.RawBucket{
				FQN:      fqn,
				Bucket:   bucketName,
				EntityID: entityID,
			}
			buckets = append(buckets, b)
		}
	}
	return s.windowBuckets(ctx, buckets)
}

func ignoreKey(ignore api.RawBuckets, key string) bool {
	for _, b := range ignore {
		if windowKey(b.FQN, b.Bucket, b.EntityID) == key {
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

			res, err := s.client.HGetAll(ctx, windowKey(b.FQN, b.Bucket, b.EntityID)).Result()
			if err != nil {
				cErr <- err
				return
			}

			rm := make(api.WindowResultMap)
			for k, v := range res {
				vv, err := strconv.ParseFloat(v, 64)
				if err != nil {
					cErr <- err
				}
				rm[api.StringToWindowFn(k)] = vv
			}
			c <- api.RawBucket{
				FQN:      b.FQN,
				Bucket:   b.Bucket,
				EntityID: b.EntityID,
				Data:     rm,
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
func (s *state) WindowBuckets(ctx context.Context, md api.Metadata, entityID string, bucketNames []string) (api.RawBuckets, error) {
	var buckets api.RawBuckets
	for _, b := range bucketNames {
		buckets = append(buckets, api.RawBucket{
			FQN:      md.FQN,
			Bucket:   b,
			EntityID: entityID,
		})
	}
	buckets, err := s.windowBuckets(ctx, buckets)
	if err != nil {
		return nil, err
	}

	return buckets, nil
}

func (s *state) getWindow(ctx context.Context, md api.Metadata, entityID string) (*api.Value, error) {
	buckets, err := s.WindowBuckets(ctx, md, entityID, api.AliveWindowBuckets(md.Staleness, md.Freshness))
	if err != nil {
		return nil, err
	}

	var avg bool
	ret := make(api.WindowResultMap)
	for _, b := range buckets {
		for _, fn := range md.Aggr {
			switch fn {
			case api.WindowFnCount, api.WindowFnSum:
				ret[fn] += b.Data[fn]
			case api.WindowFnMin:
				if _, ok := ret[fn]; !ok {
					ret[fn] = b.Data[fn]
				} else {
					ret[fn] = math.Min(ret[fn], b.Data[fn])
				}
			case api.WindowFnMax:
				if _, ok := ret[fn]; !ok {
					ret[fn] = b.Data[fn]
				} else {
					ret[fn] = math.Max(ret[fn], b.Data[fn])
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
	key := windowKey(md.FQN, bucket, entityID)

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

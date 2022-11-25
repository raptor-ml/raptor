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
	"reflect"
	"time"
)

func primitiveKey(fd api.FeatureDescriptor, keys api.Keys) (string, error) {
	e, err := keys.Encode(fd)
	if err != nil {
		return "", fmt.Errorf("failed to encode keys: %w", err)
	}
	return fmt.Sprintf("%s:%s", fd.Keys, e), nil
}

func (s *state) Get(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys) (*api.Value, error) {
	if fd.ValidWindow() {
		return s.getWindow(ctx, fd, keys)
	}
	return s.getPrimitive(ctx, fd, keys)
}

func (s *state) getPrimitive(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys) (*api.Value, error) {
	key, err := primitiveKey(fd, keys)
	if err != nil {
		return nil, err
	}

	ts, err := getTimestamp(ctx, s.client, key)
	if errors.Is(err, redis.Nil) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	var val any
	if fd.Primitive.Scalar() {
		res, err := s.client.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		val, err = api.ScalarFromString(res, fd.Primitive)
		if err != nil {
			return nil, err
		}
	} else {
		var ret []any
		res, err := s.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			return nil, err
		}
		for _, v := range res {
			v2, err := api.ScalarFromString(v, fd.Primitive.Singular())
			if err != nil {
				return nil, err
			}
			ret = append(ret, v2)
		}
		val, err = api.NormalizeAny(ret)
		if err != nil {
			return nil, err
		}
	}

	return &api.Value{
		Value:     val,
		Timestamp: *ts,
		Fresh:     time.Since(*ts) < fd.Freshness,
	}, nil
}
func (s *state) Update(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, value any, ts time.Time) error {
	if fd.ValidWindow() {
		return s.WindowAdd(ctx, fd, keys, value, ts)
	}
	if fd.Primitive.Scalar() {
		return s.Set(ctx, fd, keys, value, ts)
	}
	return s.Append(ctx, fd, keys, value, ts)
}
func (s *state) Set(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, value any, ts time.Time) error {
	if fd.ValidWindow() {
		return s.WindowAdd(ctx, fd, keys, value, ts)
	}
	if time.Since(ts) > fd.Staleness {
		return fmt.Errorf("timestamp %s is too old", ts)
	}

	key, err := primitiveKey(fd, keys)
	if err != nil {
		return err
	}

	tx := s.client.TxPipeline()

	if fd.Primitive.Scalar() {
		tx.Set(ctx, key, api.ScalarString(value), fd.Staleness)
	} else {
		tx.Del(ctx, key)
		var kv []any
		for i := 0; i < reflect.ValueOf(value).Len(); i++ {
			kv = append(kv, reflect.ValueOf(value).Index(i).Interface())
		}
		tx.RPush(ctx, key, kv...)
		if fd.Staleness > 0 {
			tx.PExpire(ctx, key, fd.Staleness)
		}
	}
	setTimestamp(ctx, tx, key, ts, fd.Staleness)

	_, err = tx.Exec(ctx)
	return err
}
func (s *state) Append(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, value any, ts time.Time) error {
	if fd.ValidWindow() {
		return fmt.Errorf("cannot append a windowed feature")
	}
	if time.Since(ts) > fd.Staleness {
		return fmt.Errorf("timestamp %s is too old", ts)
	}
	if fd.Primitive.Scalar() {
		return fmt.Errorf("`Append` only supports slices and arrays")
	}

	key, err := primitiveKey(fd, keys)
	if err != nil {
		return err
	}

	tx := s.client.TxPipeline()
	tx.RPush(ctx, key, value)
	if fd.Staleness > 0 {
		tx.PExpire(ctx, key, fd.Staleness)
	}
	setTimestamp(ctx, tx, key, ts, fd.Staleness)

	_, err = tx.Exec(ctx)
	return err
}

func (s *state) Incr(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, value any, ts time.Time) error {
	if fd.ValidWindow() {
		return fmt.Errorf("cannot increment to a windowed feature")
	}
	if time.Since(ts) > fd.Staleness {
		return fmt.Errorf("timestamp %s is too old", ts)
	}
	if !fd.Primitive.Scalar() {
		return fmt.Errorf("`Ince` only supports sclars")
	}

	key, err := primitiveKey(fd, keys)
	if err != nil {
		return err
	}

	tx := s.client.TxPipeline()
	switch v := value.(type) {
	case int:
		tx.IncrBy(ctx, key, int64(v))
	case float64:
		tx.IncrByFloat(ctx, key, v)
	default:
		return fmt.Errorf("`Incr` only supports scalar numberic values")
	}

	if fd.Staleness > 0 {
		tx.PExpire(ctx, key, fd.Staleness)
	}
	setTimestamp(ctx, tx, key, ts, fd.Staleness)

	_, err = tx.Exec(ctx)
	return err
}

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
	"reflect"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/raptor-ml/raptor/api"
)

func primitiveKey(fqn string, entityID string) string {
	return fmt.Sprintf("%s:%s", fqn, entityID)
}

func (s *state) Get(ctx context.Context, md api.Metadata, entityID string) (*api.Value, error) {
	if md.ValidWindow() {
		return s.getWindow(ctx, md, entityID)
	}
	return s.getPrimitive(ctx, md, entityID)
}

func (s *state) getPrimitive(ctx context.Context, md api.Metadata, entityID string) (*api.Value, error) {
	key := primitiveKey(md.FQN, entityID)

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
	if md.Primitive.Scalar() {
		res, err := s.client.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		val, err = api.ScalarFromString(res, md.Primitive)
		if err != nil {
			return nil, err
		}
	} else {
		res, err := s.client.LRange(ctx, key, 0, -1).Result()
		ret := make([]any, 0, len(res))
		if err != nil {
			return nil, err
		}
		for _, v := range res {
			v2, err := api.ScalarFromString(v, md.Primitive.Singular())
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
		Fresh:     time.Since(*ts) < md.Freshness,
	}, nil
}

func (s *state) Update(ctx context.Context, md api.Metadata, entityID string, value any, ts time.Time) error {
	if md.ValidWindow() {
		return s.WindowAdd(ctx, md, entityID, value, ts)
	}
	if md.Primitive.Scalar() {
		return s.Set(ctx, md, entityID, value, ts)
	}
	return s.Append(ctx, md, entityID, value, ts)
}

func (s *state) Set(ctx context.Context, md api.Metadata, entityID string, value any, ts time.Time) error {
	if md.ValidWindow() {
		return s.WindowAdd(ctx, md, entityID, value, ts)
	}
	if time.Since(ts) > md.Staleness {
		return fmt.Errorf("timestamp %s is too old", ts)
	}
	key := primitiveKey(md.FQN, entityID)

	tx := s.client.TxPipeline()

	if md.Primitive.Scalar() {
		tx.Set(ctx, key, api.ScalarString(value), md.Staleness)
	} else {
		tx.Del(ctx, key)
		reflectedSlice := reflect.ValueOf(value)
		reflectedLen := reflectedSlice.Len()
		kv := make([]any, 0, reflectedLen)
		for i := 0; i < reflectedLen; i++ {
			kv = append(kv, reflectedSlice.Index(i).Interface())
		}
		tx.RPush(ctx, key, kv...)
		if md.Staleness > 0 {
			tx.PExpire(ctx, key, md.Staleness)
		}
	}
	if err := setTimestamp(ctx, tx, key, ts, md.Staleness).Err(); err != nil {
		return err
	}

	_, err := tx.Exec(ctx)
	return err
}

func (s *state) Append(ctx context.Context, md api.Metadata, entityID string, value any, ts time.Time) error {
	if md.ValidWindow() {
		return fmt.Errorf("cannot append a windowed feature")
	}
	if time.Since(ts) > md.Staleness {
		return fmt.Errorf("timestamp %s is too old", ts)
	}
	if md.Primitive.Scalar() {
		return fmt.Errorf("`Append` only supports slices and arrays")
	}

	key := primitiveKey(md.FQN, entityID)

	tx := s.client.TxPipeline()
	tx.RPush(ctx, key, value)
	if md.Staleness > 0 {
		tx.PExpire(ctx, key, md.Staleness)
	}
	if err := setTimestamp(ctx, tx, key, ts, md.Staleness).Err(); err != nil {
		return err
	}

	_, err := tx.Exec(ctx)
	return err
}

func (s *state) Incr(ctx context.Context, md api.Metadata, entityID string, value any, ts time.Time) error {
	if md.ValidWindow() {
		return fmt.Errorf("cannot increment to a windowed feature")
	}
	if time.Since(ts) > md.Staleness {
		return fmt.Errorf("timestamp %s is too old", ts)
	}
	if !md.Primitive.Scalar() {
		return fmt.Errorf("`Ince` only supports sclars")
	}
	key := primitiveKey(md.FQN, entityID)

	tx := s.client.TxPipeline()
	switch v := value.(type) {
	case int:
		tx.IncrBy(ctx, key, int64(v))
	case float64:
		tx.IncrByFloat(ctx, key, v)
	default:
		return fmt.Errorf("`Incr` only supports scalar numberic values")
	}

	if md.Staleness > 0 {
		tx.PExpire(ctx, key, md.Staleness)
	}
	if err := setTimestamp(ctx, tx, key, ts, md.Staleness).Err(); err != nil {
		return err
	}

	_, err := tx.Exec(ctx)
	return err
}

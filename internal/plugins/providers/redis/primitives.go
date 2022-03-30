package redis

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/pkg/api"
	"reflect"
	"time"
)

func (s *state) Get(ctx context.Context, md api.Metadata, entityID string) (*api.Value, error) {
	if md.ValidWindow() {
		return s.getWindow(ctx, md, entityID)
	}
	return s.getPrimitive(ctx, md, entityID)
}

func (s *state) getPrimitive(ctx context.Context, md api.Metadata, entityID string) (*api.Value, error) {
	key := key(md.FQN, entityID)

	ts, err := getTimestamp(ctx, s.client, key)
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
		var ret []any
		res, err := s.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			return nil, err
		}
		for _, v := range res {
			v2, err := api.ScalarFromString(v, md.Primitive)
			if err != nil {
				return nil, err
			}
			ret = append(ret, v2)
		}
		val = ret
	}

	return &api.Value{
		Value:     val,
		Timestamp: *ts,
		Fresh:     time.Now().Sub(*ts) < md.Freshness,
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
	if time.Now().Sub(ts) > md.Staleness {
		return fmt.Errorf("timestamp %s is too old", ts)
	}
	key := key(md.FQN, entityID)

	tx := s.client.TxPipeline()

	if md.Primitive.Scalar() {
		tx.Set(ctx, key, api.ScalarString(value), md.Staleness)
	} else {
		tx.Del(ctx, key)
		var kv []any
		for i := 0; i < reflect.ValueOf(value).Len(); i++ {
			kv = append(kv, reflect.ValueOf(value).Index(i).Interface())
		}
		tx.RPush(ctx, key, kv...)
		if md.Staleness > 0 {
			tx.PExpire(ctx, key, md.Staleness)
		}
	}
	setTimestamp(ctx, tx, key, ts, md.Staleness)

	_, err := tx.Exec(ctx)
	return err
}
func (s *state) Append(ctx context.Context, md api.Metadata, entityID string, value any, ts time.Time) error {
	if md.ValidWindow() {
		return fmt.Errorf("cannot append a windowed feature")
	}
	if time.Now().Sub(ts) > md.Staleness {
		return fmt.Errorf("timestamp %s is too old", ts)
	}
	if md.Primitive.Scalar() {
		return fmt.Errorf("`Append` only supports slices and arrays")
	}

	key := key(md.FQN, entityID)

	tx := s.client.TxPipeline()
	tx.RPush(ctx, key, value)
	if md.Staleness > 0 {
		tx.PExpire(ctx, key, md.Staleness)
	}
	setTimestamp(ctx, tx, key, ts, md.Staleness)

	_, err := tx.Exec(ctx)
	return err
}

func (s *state) Incr(ctx context.Context, md api.Metadata, entityID string, value any, ts time.Time) error {
	if md.ValidWindow() {
		return fmt.Errorf("cannot increment to a windowed feature")
	}
	if time.Now().Sub(ts) > md.Staleness {
		return fmt.Errorf("timestamp %s is too old", ts)
	}
	if !md.Primitive.Scalar() {
		return fmt.Errorf("`Ince` only supports sclars")
	}
	key := key(md.FQN, entityID)

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
	setTimestamp(ctx, tx, key, ts, md.Staleness)

	_, err := tx.Exec(ctx)
	return err
}

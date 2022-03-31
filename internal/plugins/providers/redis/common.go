package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/natun-ai/natun/internal/plugin"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"strconv"
	"time"
)

const pluginName = "redis"

func init() {
	plugin.Configurers.Register(pluginName, BindConfig)
	plugin.StateFactories.Register(pluginName, StateFactory)
}

type state struct {
	client redis.UniversalClient
}

func (s *state) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func redisClient(viper *viper.Viper) (redis.UniversalClient, error) {
	// Initialize redis client
	var redisTLS *tls.Config = nil
	if viper.GetBool("redis-tls") {
		redisTLS = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	addrs := viper.GetStringSlice("redis")
	if len(addrs) == 0 {
		return nil, fmt.Errorf("redis: no redis addresses provided")
	}

	return redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:            addrs,
		DB:               viper.GetInt("redis-db"),
		Password:         viper.GetString("redis-pass"),
		Username:         viper.GetString("redis-user"),
		SentinelUsername: viper.GetString("redis-sentinel-user"),
		SentinelPassword: viper.GetString("redis-sentinel-pass"),
		MasterName:       viper.GetString("redis-master"),
		TLSConfig:        redisTLS,
	}), nil
}

func StateFactory(viper *viper.Viper) (api.State, error) {
	rc, err := redisClient(viper)
	if err != nil {
		return nil, err
	}
	// Load Lua scripts in advance. This is useful in case we have permissions issue, so we'll detect it in advance.
	err = scripts.Load(rc)
	if err != nil {
		return nil, err
	}

	return &state{rc}, nil
}
func BindConfig(set *pflag.FlagSet) error {
	set.StringArrayP("redis", "r", []string{}, "Redis URI")
	set.String("redis-user", "", "Redis username")
	set.String("redis-pass", "", "Redis password")
	set.String("redis-sentinel-user", "", "Redis Sentinel username")
	set.String("redis-sentinel-pass", "", "Redis Sentinel password")
	set.String("redis-master", "", "Redis Sentinel master name")
	set.Bool("redis-tls", false, "Enable TLS for Redis")
	set.Int("redis-db", 0, "Redis DB")
	return nil
}

func setTimestamp(ctx context.Context, tx redis.Cmdable, key string, ts time.Time, ttl time.Duration) *redis.Cmd {
	key = fmt.Sprintf("%s:ts", key)
	dur := int64(ttl / time.Millisecond)
	if ttl < time.Millisecond {
		dur = 1
	}
	return luaMax.Run(ctx, tx, []string{key}, ts.UnixMicro(), dur)
}
func setTimestampExpireAt(ctx context.Context, tx redis.Cmdable, key string, ts time.Time, xat time.Time) *redis.Cmd {
	return luaMax.Run(ctx, tx, []string{key}, ts.UnixMicro(), xat.UnixMilli())
}
func getTimestamp(ctx context.Context, tx redis.Cmdable, key string) (*time.Time, error) {
	s, err := tx.Get(ctx, fmt.Sprintf("%s:ts", key)).Result()
	if err != nil {
		return nil, fmt.Errorf("unable to fetch timestamp for primitiveKey %s: %w", key, err)
	}

	nts, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse timestamp for primitiveKey %s: %w", key, err)
	}
	ts := time.UnixMicro(nts)
	return &ts, nil
}

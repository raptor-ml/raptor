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

package snowflake

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/natun-ai/natun/api"
	"github.com/natun-ai/natun/pkg/plugins"
	sf "github.com/snowflakedb/gosnowflake"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/xitongsys/parquet-go/types"
	"strings"
	"time"
)

const pluginName = "snowflake"

func init() {
	plugins.Configurers.Register(pluginName, BindConfig)
	plugins.HistoricalWriterFactories.Register(pluginName, HistoricalWriterFactory)
}

func BindConfig(set *pflag.FlagSet) error {
	set.String("snowflake-uri", "", "Snowflake DSN URI")
	return nil
}

func HistoricalWriterFactory(viper *viper.Viper) (api.HistoricalWriter, error) {
	db, err := sql.Open("snowflake", viper.GetString("snowflake-uri"))
	if err != nil {
		return nil, fmt.Errorf("failed to open snowflake connection: %w", err)
	}
	const create = `CREATE TABLE IF NOT EXISTS historical (
		id number autoincrement start 1 increment 1,
		fqn string(255) not null,
		entity_id string(255) not null,
		value variant not null,
		timestamp timestamp_ltz	not null,
		bucket string(10),
		bucket_active boolean
	);`
	_, err = db.Exec(create)
	if err != nil {
		return nil, fmt.Errorf("failed to create snowflake table: %w", err)
	}
	return &snowflakeWriter{db: db}, nil
}

type snowflakeWriter struct {
	db *sql.DB
}

func (sw *snowflakeWriter) Commit(ctx context.Context, wn api.WriteNotification) error {
	stmt, err := sw.db.PrepareContext(ctx, `INSERT INTO historical (fqn, entity_id, value, timestamp, bucket, bucket_active) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare snowflake insert: %w", err)
	}

	var valTyp []byte
	var val any
	var bucket *string
	var alive *bool
	if wn.Bucket != "" {
		b := strings.TrimSuffix(wn.Bucket, api.AliveMarker)
		bucket = &b
		alv := false
		if isAlive(wn) {
			alv = true
		}
		alive = &alv

		wrm := api.ToLowLevelValue[api.WindowResultMap](wn.Value.Value)
		valTyp = sf.DataTypeObject
		v := make(map[string]float64)
		for k, vv := range wrm {
			v[k.String()] = vv
		}
		val = v
	} else {
		switch api.TypeDetect(wn.Value.Value) {
		case api.PrimitiveTypeString:
			valTyp, val = sf.DataTypeFixed, api.ToLowLevelValue[string](wn.Value.Value)
		case api.PrimitiveTypeInteger:
			valTyp, val = sf.DataTypeVariant, int64(api.ToLowLevelValue[int](wn.Value.Value))
		case api.PrimitiveTypeFloat:
			valTyp, val = sf.DataTypeReal, api.ToLowLevelValue[float64](wn.Value.Value)
		case api.PrimitiveTypeTimestamp:
			valTyp, val = sf.DataTypeTimestampLtz, api.ToLowLevelValue[time.Time](wn.Value.Value)
		case api.PrimitiveTypeStringList:
			valTyp, val = sf.DataTypeArray, sf.Array(api.ToLowLevelValue[[]string](wn.Value.Value))

		case api.PrimitiveTypeIntegerList:
			v := api.ToLowLevelValue[[]int](wn.Value.Value)
			var l []int64
			for _, i := range v {
				l = append(l, int64(i))
			}
			valTyp, val = sf.DataTypeArray, sf.Array(l)
		case api.PrimitiveTypeFloatList:
			valTyp, val = sf.DataTypeArray, sf.Array(api.ToLowLevelValue[[]float64](wn.Value.Value))
		case api.PrimitiveTypeTimestampList:
			v := api.ToLowLevelValue[[]time.Time](wn.Value.Value)
			var l []int64
			for _, t := range v {
				l = append(l, types.TimeToTIMESTAMP_MICROS(t, false))
			}
			valTyp, val = sf.DataTypeArray, sf.Array(l)
		}
	}
	_, err = stmt.ExecContext(ctx, wn.FQN, wn.EntityID, valTyp, val, sf.DataTypeTimestampLtz, wn.Value.Timestamp, bucket, alive)
	return err
}
func (sw *snowflakeWriter) Flush(ctx context.Context, fqn string) error { return nil }
func (sw *snowflakeWriter) FlushAll(context.Context) error              { return nil }
func (sw *snowflakeWriter) Close(ctx context.Context) error {
	return sw.db.Close()
}

func isAlive(wn api.WriteNotification) bool {
	return strings.HasSuffix(wn.Bucket, api.AliveMarker)
}

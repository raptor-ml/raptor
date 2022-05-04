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
	"encoding/json"
	"fmt"
	"github.com/natun-ai/natun/api"
	"github.com/natun-ai/natun/pkg/plugins"
	sf "github.com/snowflakedb/gosnowflake"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"net/url"
	"strings"
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
	uri := viper.GetString("snowflake-uri")
	if !strings.HasPrefix(uri, "snowflake://") {
		uri = "snowflake://" + uri
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse snowflake uri: %w", err)
	}
	if u.Query().Get("warehouse") == "" {
		return nil, fmt.Errorf("warehouse is required")
	}
	if u.Scheme != "snowflake" {
		return nil, fmt.Errorf("scheme must be snowflake")
	}
	dsn := strings.TrimPrefix(u.String(), "snowflake://")

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open snowflake connection: %w", err)
	}

	sw := &snowflakeWriter{db: db, config: u.Query()}
	err = sw.init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize snowflake writer: %w", err)
	}
	return &snowflakeWriter{db: db}, nil
}

type snowflakeWriter struct {
	db     *sql.DB
	config url.Values
}

func (sw *snowflakeWriter) Commit(ctx context.Context, wn api.WriteNotification) error {
	q := `INSERT INTO historical (fqn, entity_id, value, timestamp, bucket, bucket_active) SELECT ?, ?, to_variant(%s), ?, ?, ?`
	var val any
	var bucket *string
	var alive *bool
	if wn.Bucket != "" {
		bucket = &wn.Bucket
		alive = &wn.ActiveBucket

		wrm := api.ToLowLevelValue[api.WindowResultMap](wn.Value.Value)
		v := make(map[string]float64)
		for k, vv := range wrm {
			v[k.String()] = vv
		}
		rawJSON, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal snowflake value: %w", err)
		}
		q = fmt.Sprintf(q, "parse_json(%s)")
		val = string(rawJSON)
	} else {
		val = wn.Value.Value
		if !api.TypeDetect(val).Scalar() {
			rawJSON, err := json.Marshal(val)
			if err != nil {
				return fmt.Errorf("failed to marshal snowflake value: %w", err)
			}
			q = fmt.Sprintf(q, "parse_json(%s)")
			val = string(rawJSON)
		}
	}

	stmt, err := sw.db.PrepareContext(ctx, fmt.Sprintf(q, "?"))
	if err != nil {
		return fmt.Errorf("failed to prepare snowflake insert: %w", err)
	}
	_, err = stmt.ExecContext(ctx, wn.FQN, wn.EntityID, val, sf.DataTypeTimestampLtz, wn.Value.Timestamp, bucket, alive)
	return err
}
func (sw *snowflakeWriter) Flush(ctx context.Context, fqn string) error { return nil }
func (sw *snowflakeWriter) FlushAll(context.Context) error              { return nil }
func (sw *snowflakeWriter) Close(ctx context.Context) error {
	return sw.db.Close()
}

func (sw *snowflakeWriter) init() error {
	err := sw.createTable()
	if err != nil {
		return fmt.Errorf("failed to create snowflake table: %w", err)
	}

	err = sw.createTask()
	if err != nil {
		return fmt.Errorf("failed to verify snowflake task: %w", err)
	}
	return nil
}
func (sw *snowflakeWriter) createTable() error {
	const create = `CREATE TABLE IF NOT EXISTS historical (
		id number autoincrement start 1 increment 1,
		fqn string(255) not null,
		entity_id string(255) not null,
		value variant not null,
		timestamp timestamp_ltz	not null,
		bucket string(10),
		bucket_active boolean,
		UNIQUE(fqn, entity_id, value, timestamp, bucket, bucket_active)
	);`
	_, err := sw.db.Exec(create)
	return err
}

func (sw *snowflakeWriter) createTask() error {
	const cleanupTask = `create task if not exists active_buckets_cleanup
    schedule = '60 minute'
    allow_overlapping_execution = false
    warehouse = '%s'
as
    merge into historical as target using historical as source
        on target.fqn = source.fqn
        and target.entity_id = source.entity_id
        and target.bucket = source.bucket
        when matched and target.bucket is not null and target.bucket_active = true and source.bucket_active = false
        then delete`
	_, err := sw.db.Exec(fmt.Sprintf(cleanupTask, sw.config.Get("warehouse")))
	if err != nil {
		return fmt.Errorf("failed to create snowflake task: %w", err)
	}

	const resumeTask = `ALTER TASK active_buckets_cleanup RESUME`
	_, err = sw.db.Exec(resumeTask)
	if err != nil {
		return fmt.Errorf("failed to resume snowflake task: %w", err)
	}
	return nil
}

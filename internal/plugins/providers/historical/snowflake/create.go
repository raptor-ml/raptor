/*
 * Copyright (c) 2022 RaptorML authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package snowflake

import "fmt"

func (sw *snowflakeWriter) createTable() error {
	const create = `CREATE TABLE IF NOT EXISTS %s(
    id            number autoincrement start 1 increment 1,
    fqn           string(255)   not null,
    entity_id     string(255)   not null,
    value         variant       not null,
    timestamp     timestamp_ltz not null,
    bucket        string(10),
    bucket_active boolean,
    UNIQUE (fqn, entity_id, value, timestamp, bucket, bucket_active)
) CLUSTER BY (fqn, timestamp);`
	_, err := sw.db.Exec(fmt.Sprintf(create, featuresTable))
	return err
}

func (sw *snowflakeWriter) createTask() error {
	const cleanupTask = `CREATE TASK IF NOT EXISTS %s_cleanup
    SCHEDULE = '60 minute'
    ALLOW_OVERLAPPING_EXECUTION = FALSE
    WAREHOUSE = '%s'
	COMMENT = 'Remove active buckets that were finalized'
    AS
        MERGE INTO historical AS target USING %s AS source
            ON target.fqn = source.fqn
                AND target.entity_id = source.entity_id
                AND target.bucket = source.bucket
            WHEN MATCHED AND target.bucket IS NOT NULL AND target.bucket_active = TRUE AND source.bucket_active = FALSE
                THEN DELETE;`
	_, err := sw.db.Exec(fmt.Sprintf(cleanupTask, featuresTable, sw.config.Get("warehouse"), featuresTable))
	if err != nil {
		return fmt.Errorf("failed to create snowflake task: %w", err)
	}

	const resumeTask = `ALTER TASK %s_cleanup RESUME`
	_, err = sw.db.Exec(fmt.Sprintf(resumeTask, featuresTable))
	if err != nil {
		return fmt.Errorf("failed to resume snowflake task: %w", err)
	}
	return nil
}

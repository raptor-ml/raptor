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

package plugins

import (
	// register all builder plugins
	_ "github.com/natun-ai/natun/internal/plugins/builders/expression"
	_ "github.com/natun-ai/natun/internal/plugins/builders/rest"
	_ "github.com/natun-ai/natun/internal/plugins/builders/streaming"

	// register all historical provider plugins
	_ "github.com/natun-ai/natun/internal/plugins/providers/historical/parquet/aws"
	_ "github.com/natun-ai/natun/internal/plugins/providers/historical/snowflake"

	// register all state provider plugins
	_ "github.com/natun-ai/natun/internal/plugins/providers/state/redis"
)

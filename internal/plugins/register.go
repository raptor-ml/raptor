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

package plugins

import (
	_ "github.com/raptor-ml/raptor/internal/plugins/builders/model"
	// register all builder plugins
	_ "github.com/raptor-ml/raptor/internal/plugins/builders/headless"
	_ "github.com/raptor-ml/raptor/internal/plugins/builders/rest"
	_ "github.com/raptor-ml/raptor/internal/plugins/builders/streaming"

	// register all historical provider plugins
	_ "github.com/raptor-ml/raptor/internal/plugins/providers/historical/parquet/s3"
	_ "github.com/raptor-ml/raptor/internal/plugins/providers/historical/snowflake"

	// register all state provider plugins
	_ "github.com/raptor-ml/raptor/internal/plugins/providers/state/redis"
)

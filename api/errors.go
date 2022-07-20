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

package api

import "fmt"

// ErrUnsupportedPrimitiveError is returned when a primitive is not supported.
var ErrUnsupportedPrimitiveError = fmt.Errorf("unsupported primitive")

// ErrUnsupportedAggrError is returned when an aggregate function is not supported.
var ErrUnsupportedAggrError = fmt.Errorf("unsupported aggr")

// ErrFeatureNotFound is returned when a feature is not found in the Core's engine manager.
var ErrFeatureNotFound = fmt.Errorf("feature not found")

// ErrFeatureAlreadyExists is returned when a feature is already registered in the Core's engine manager.
var ErrFeatureAlreadyExists = fmt.Errorf("feature already exists")

// ErrInvalidPipelineContext is returned when the context is invalid for pipelining.
var ErrInvalidPipelineContext = fmt.Errorf("invalid pipeline context")

package errors

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

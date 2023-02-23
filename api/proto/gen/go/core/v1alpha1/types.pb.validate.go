// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: core/v1alpha1/types.proto

package corev1alpha1

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/anypb"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = anypb.Any{}
	_ = sort.Sort
)

// Validate checks the field values on Scalar with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Scalar) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Scalar with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in ScalarMultiError, or nil if none found.
func (m *Scalar) ValidateAll() error {
	return m.validate(true)
}

func (m *Scalar) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	switch v := m.Value.(type) {
	case *Scalar_StringValue:
		if v == nil {
			err := ScalarValidationError{
				field:  "Value",
				reason: "oneof value cannot be a typed-nil",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}
		// no validation rules for StringValue
	case *Scalar_IntValue:
		if v == nil {
			err := ScalarValidationError{
				field:  "Value",
				reason: "oneof value cannot be a typed-nil",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}
		// no validation rules for IntValue
	case *Scalar_FloatValue:
		if v == nil {
			err := ScalarValidationError{
				field:  "Value",
				reason: "oneof value cannot be a typed-nil",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}
		// no validation rules for FloatValue
	case *Scalar_BoolValue:
		if v == nil {
			err := ScalarValidationError{
				field:  "Value",
				reason: "oneof value cannot be a typed-nil",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}
		// no validation rules for BoolValue
	case *Scalar_TimestampValue:
		if v == nil {
			err := ScalarValidationError{
				field:  "Value",
				reason: "oneof value cannot be a typed-nil",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

		if all {
			switch v := interface{}(m.GetTimestampValue()).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, ScalarValidationError{
						field:  "TimestampValue",
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, ScalarValidationError{
						field:  "TimestampValue",
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(m.GetTimestampValue()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return ScalarValidationError{
					field:  "TimestampValue",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	default:
		_ = v // ensures v is used
	}

	if len(errors) > 0 {
		return ScalarMultiError(errors)
	}

	return nil
}

// ScalarMultiError is an error wrapping multiple validation errors returned by
// Scalar.ValidateAll() if the designated constraints aren't met.
type ScalarMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ScalarMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ScalarMultiError) AllErrors() []error { return m }

// ScalarValidationError is the validation error returned by Scalar.Validate if
// the designated constraints aren't met.
type ScalarValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ScalarValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ScalarValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ScalarValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ScalarValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ScalarValidationError) ErrorName() string { return "ScalarValidationError" }

// Error satisfies the builtin error interface
func (e ScalarValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sScalar.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ScalarValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ScalarValidationError{}

// Validate checks the field values on List with the rules defined in the proto
// definition for this message. If any rules are violated, the first error
// encountered is returned, or nil if there are no violations.
func (m *List) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on List with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in ListMultiError, or nil if none found.
func (m *List) ValidateAll() error {
	return m.validate(true)
}

func (m *List) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	for idx, item := range m.GetValues() {
		_, _ = idx, item

		if all {
			switch v := interface{}(item).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, ListValidationError{
						field:  fmt.Sprintf("Values[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, ListValidationError{
						field:  fmt.Sprintf("Values[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return ListValidationError{
					field:  fmt.Sprintf("Values[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	if len(errors) > 0 {
		return ListMultiError(errors)
	}

	return nil
}

// ListMultiError is an error wrapping multiple validation errors returned by
// List.ValidateAll() if the designated constraints aren't met.
type ListMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ListMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ListMultiError) AllErrors() []error { return m }

// ListValidationError is the validation error returned by List.Validate if the
// designated constraints aren't met.
type ListValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ListValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ListValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ListValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ListValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ListValidationError) ErrorName() string { return "ListValidationError" }

// Error satisfies the builtin error interface
func (e ListValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sList.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ListValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ListValidationError{}

// Validate checks the field values on Value with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Value) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Value with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in ValueMultiError, or nil if none found.
func (m *Value) ValidateAll() error {
	return m.validate(true)
}

func (m *Value) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	switch v := m.Value.(type) {
	case *Value_ScalarValue:
		if v == nil {
			err := ValueValidationError{
				field:  "Value",
				reason: "oneof value cannot be a typed-nil",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

		if all {
			switch v := interface{}(m.GetScalarValue()).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, ValueValidationError{
						field:  "ScalarValue",
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, ValueValidationError{
						field:  "ScalarValue",
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(m.GetScalarValue()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return ValueValidationError{
					field:  "ScalarValue",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	case *Value_ListValue:
		if v == nil {
			err := ValueValidationError{
				field:  "Value",
				reason: "oneof value cannot be a typed-nil",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

		if all {
			switch v := interface{}(m.GetListValue()).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, ValueValidationError{
						field:  "ListValue",
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, ValueValidationError{
						field:  "ListValue",
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(m.GetListValue()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return ValueValidationError{
					field:  "ListValue",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	default:
		_ = v // ensures v is used
	}

	if len(errors) > 0 {
		return ValueMultiError(errors)
	}

	return nil
}

// ValueMultiError is an error wrapping multiple validation errors returned by
// Value.ValidateAll() if the designated constraints aren't met.
type ValueMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ValueMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ValueMultiError) AllErrors() []error { return m }

// ValueValidationError is the validation error returned by Value.Validate if
// the designated constraints aren't met.
type ValueValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ValueValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ValueValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ValueValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ValueValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ValueValidationError) ErrorName() string { return "ValueValidationError" }

// Error satisfies the builtin error interface
func (e ValueValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sValue.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ValueValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ValueValidationError{}

// Validate checks the field values on ObjectReference with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *ObjectReference) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on ObjectReference with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// ObjectReferenceMultiError, or nil if none found.
func (m *ObjectReference) ValidateAll() error {
	return m.validate(true)
}

func (m *ObjectReference) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Name

	// no validation rules for Namespace

	if len(errors) > 0 {
		return ObjectReferenceMultiError(errors)
	}

	return nil
}

// ObjectReferenceMultiError is an error wrapping multiple validation errors
// returned by ObjectReference.ValidateAll() if the designated constraints
// aren't met.
type ObjectReferenceMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ObjectReferenceMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ObjectReferenceMultiError) AllErrors() []error { return m }

// ObjectReferenceValidationError is the validation error returned by
// ObjectReference.Validate if the designated constraints aren't met.
type ObjectReferenceValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ObjectReferenceValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ObjectReferenceValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ObjectReferenceValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ObjectReferenceValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ObjectReferenceValidationError) ErrorName() string { return "ObjectReferenceValidationError" }

// Error satisfies the builtin error interface
func (e ObjectReferenceValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sObjectReference.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ObjectReferenceValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ObjectReferenceValidationError{}

// Validate checks the field values on KeepPrevious with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *KeepPrevious) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on KeepPrevious with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in KeepPreviousMultiError, or
// nil if none found.
func (m *KeepPrevious) ValidateAll() error {
	return m.validate(true)
}

func (m *KeepPrevious) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Versions

	if all {
		switch v := interface{}(m.GetOver()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, KeepPreviousValidationError{
					field:  "Over",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, KeepPreviousValidationError{
					field:  "Over",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetOver()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return KeepPreviousValidationError{
				field:  "Over",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if len(errors) > 0 {
		return KeepPreviousMultiError(errors)
	}

	return nil
}

// KeepPreviousMultiError is an error wrapping multiple validation errors
// returned by KeepPrevious.ValidateAll() if the designated constraints aren't met.
type KeepPreviousMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m KeepPreviousMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m KeepPreviousMultiError) AllErrors() []error { return m }

// KeepPreviousValidationError is the validation error returned by
// KeepPrevious.Validate if the designated constraints aren't met.
type KeepPreviousValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e KeepPreviousValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e KeepPreviousValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e KeepPreviousValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e KeepPreviousValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e KeepPreviousValidationError) ErrorName() string { return "KeepPreviousValidationError" }

// Error satisfies the builtin error interface
func (e KeepPreviousValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sKeepPrevious.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = KeepPreviousValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = KeepPreviousValidationError{}

// Validate checks the field values on FeatureDescriptor with the rules defined
// in the proto definition for this message. If any rules are violated, the
// first error encountered is returned, or nil if there are no violations.
func (m *FeatureDescriptor) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on FeatureDescriptor with the rules
// defined in the proto definition for this message. If any rules are
// violated, the result is a list of violation errors wrapped in
// FeatureDescriptorMultiError, or nil if none found.
func (m *FeatureDescriptor) ValidateAll() error {
	return m.validate(true)
}

func (m *FeatureDescriptor) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if !_FeatureDescriptor_Fqn_Pattern.MatchString(m.GetFqn()) {
		err := FeatureDescriptorValidationError{
			field:  "Fqn",
			reason: "value does not match regex pattern \"(i?)^([a0-z9\\\\-\\\\.]*)(\\\\[([a0-z9])*\\\\])?$\"",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if _, ok := Primitive_name[int32(m.GetPrimitive())]; !ok {
		err := FeatureDescriptorValidationError{
			field:  "Primitive",
			reason: "value must be one of the defined enum values",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	_FeatureDescriptor_Aggr_Unique := make(map[AggrFn]struct{}, len(m.GetAggr()))

	for idx, item := range m.GetAggr() {
		_, _ = idx, item

		if _, exists := _FeatureDescriptor_Aggr_Unique[item]; exists {
			err := FeatureDescriptorValidationError{
				field:  fmt.Sprintf("Aggr[%v]", idx),
				reason: "repeated value must contain unique items",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		} else {
			_FeatureDescriptor_Aggr_Unique[item] = struct{}{}
		}

		if _, ok := AggrFn_name[int32(item)]; !ok {
			err := FeatureDescriptorValidationError{
				field:  fmt.Sprintf("Aggr[%v]", idx),
				reason: "value must be one of the defined enum values",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

	}

	if all {
		switch v := interface{}(m.GetFreshness()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, FeatureDescriptorValidationError{
					field:  "Freshness",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, FeatureDescriptorValidationError{
					field:  "Freshness",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetFreshness()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return FeatureDescriptorValidationError{
				field:  "Freshness",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if all {
		switch v := interface{}(m.GetStaleness()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, FeatureDescriptorValidationError{
					field:  "Staleness",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, FeatureDescriptorValidationError{
					field:  "Staleness",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetStaleness()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return FeatureDescriptorValidationError{
				field:  "Staleness",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if all {
		switch v := interface{}(m.GetTimeout()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, FeatureDescriptorValidationError{
					field:  "Timeout",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, FeatureDescriptorValidationError{
					field:  "Timeout",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetTimeout()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return FeatureDescriptorValidationError{
				field:  "Timeout",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	// no validation rules for Builder

	// no validation rules for DataSource

	// no validation rules for RuntimeEnv

	if m.KeepPrevious != nil {

		if all {
			switch v := interface{}(m.GetKeepPrevious()).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, FeatureDescriptorValidationError{
						field:  "KeepPrevious",
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, FeatureDescriptorValidationError{
						field:  "KeepPrevious",
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(m.GetKeepPrevious()).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return FeatureDescriptorValidationError{
					field:  "KeepPrevious",
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	if len(errors) > 0 {
		return FeatureDescriptorMultiError(errors)
	}

	return nil
}

// FeatureDescriptorMultiError is an error wrapping multiple validation errors
// returned by FeatureDescriptor.ValidateAll() if the designated constraints
// aren't met.
type FeatureDescriptorMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m FeatureDescriptorMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m FeatureDescriptorMultiError) AllErrors() []error { return m }

// FeatureDescriptorValidationError is the validation error returned by
// FeatureDescriptor.Validate if the designated constraints aren't met.
type FeatureDescriptorValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e FeatureDescriptorValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e FeatureDescriptorValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e FeatureDescriptorValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e FeatureDescriptorValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e FeatureDescriptorValidationError) ErrorName() string {
	return "FeatureDescriptorValidationError"
}

// Error satisfies the builtin error interface
func (e FeatureDescriptorValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sFeatureDescriptor.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = FeatureDescriptorValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = FeatureDescriptorValidationError{}

var _FeatureDescriptor_Fqn_Pattern = regexp.MustCompile("(i?)^([a0-z9\\-\\.]*)(\\[([a0-z9])*\\])?$")

// Validate checks the field values on FeatureValue with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *FeatureValue) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on FeatureValue with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in FeatureValueMultiError, or
// nil if none found.
func (m *FeatureValue) ValidateAll() error {
	return m.validate(true)
}

func (m *FeatureValue) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if !_FeatureValue_Fqn_Pattern.MatchString(m.GetFqn()) {
		err := FeatureValueValidationError{
			field:  "Fqn",
			reason: "value does not match regex pattern \"(i?)^([a0-z9\\\\-\\\\.]*)(\\\\[([a0-z9])*\\\\])?$\"",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	// no validation rules for Keys

	if all {
		switch v := interface{}(m.GetValue()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, FeatureValueValidationError{
					field:  "Value",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, FeatureValueValidationError{
					field:  "Value",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetValue()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return FeatureValueValidationError{
				field:  "Value",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if all {
		switch v := interface{}(m.GetTimestamp()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, FeatureValueValidationError{
					field:  "Timestamp",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, FeatureValueValidationError{
					field:  "Timestamp",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetTimestamp()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return FeatureValueValidationError{
				field:  "Timestamp",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	// no validation rules for Fresh

	if len(errors) > 0 {
		return FeatureValueMultiError(errors)
	}

	return nil
}

// FeatureValueMultiError is an error wrapping multiple validation errors
// returned by FeatureValue.ValidateAll() if the designated constraints aren't met.
type FeatureValueMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m FeatureValueMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m FeatureValueMultiError) AllErrors() []error { return m }

// FeatureValueValidationError is the validation error returned by
// FeatureValue.Validate if the designated constraints aren't met.
type FeatureValueValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e FeatureValueValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e FeatureValueValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e FeatureValueValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e FeatureValueValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e FeatureValueValidationError) ErrorName() string { return "FeatureValueValidationError" }

// Error satisfies the builtin error interface
func (e FeatureValueValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sFeatureValue.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = FeatureValueValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = FeatureValueValidationError{}

var _FeatureValue_Fqn_Pattern = regexp.MustCompile("(i?)^([a0-z9\\-\\.]*)(\\[([a0-z9])*\\])?$")
// Package odata builds the constrained OData query surface used by Nutanix list operations.
package odata

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maximumPage = int64(1<<31 - 1)

var (
	// ErrInvalidListQuery identifies a query outside the approved list contract.
	ErrInvalidListQuery = errors.New("nutanix list query is invalid")
)

// ListOptions contains the caller-supplied inputs shared by list operations.
type ListOptions struct {
	Page    *int64
	Limit   *int64
	Filter  *string
	OrderBy *string
	Select  *string
	Expand  *string
}

// QueryOption identifies one caller-supplied list option without naming a consumer attribute.
type QueryOption uint8

const (
	QueryOptionPage QueryOption = iota + 1
	QueryOptionLimit
	QueryOptionFilter
	QueryOptionOrderBy
	QueryOptionSelect
	QueryOptionExpand
)

// InvalidOptionError identifies one invalid caller option without retaining its value.
type InvalidOptionError struct {
	option QueryOption
}

// Error returns the redacted list-query error text.
func (*InvalidOptionError) Error() string {
	return ErrInvalidListQuery.Error()
}

// Unwrap preserves errors.Is compatibility with ErrInvalidListQuery.
func (*InvalidOptionError) Unwrap() error {
	return ErrInvalidListQuery
}

// Option returns the neutral option identity.
func (e *InvalidOptionError) Option() QueryOption {
	return e.option
}

// Policy locks the projections and optional query features for one operation.
type Policy struct {
	RequiredSelect []string
	RequiredExpand []string
	AllowExpand    bool
}

// Query holds immutable wire and caller-identity query values.
type Query struct {
	wire     url.Values
	identity url.Values
}

// Format prevents caller filters and projections from entering diagnostics.
func (Query) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("odata.Query(redacted)"))
}

// Build validates caller options and constructs the exact wire projection.
func Build(options ListOptions, policy Policy) (Query, error) {
	if err := ValidateOptions(options); err != nil {
		return Query{}, err
	}
	if !policy.AllowExpand && (options.Expand != nil || len(policy.RequiredExpand) > 0) {
		return Query{}, ErrInvalidListQuery
	}

	selectValue, err := mergeProjection(options.Select, policy.RequiredSelect)
	if err != nil || selectValue == "" {
		return Query{}, ErrInvalidListQuery
	}
	expandValue, err := mergeProjection(options.Expand, policy.RequiredExpand)
	if err != nil {
		return Query{}, ErrInvalidListQuery
	}

	identity := make(url.Values)
	wire := make(url.Values)
	addInteger(wire, identity, "$page", options.Page)
	addInteger(wire, identity, "$limit", options.Limit)
	addString(wire, identity, "$filter", options.Filter)
	addString(wire, identity, "$orderby", options.OrderBy)
	addIdentityString(identity, "$select", options.Select)
	addIdentityString(identity, "$expand", options.Expand)
	wire.Set("$select", selectValue)
	if expandValue != "" {
		wire.Set("$expand", expandValue)
	}

	return Query{wire: wire, identity: identity}, nil
}

// ValidateOptions checks caller-supplied list syntax independently of namespace policy.
func ValidateOptions(options ListOptions) error {
	if options.Page != nil && (*options.Page < 0 || *options.Page > maximumPage) {
		return newInvalidOptionError(QueryOptionPage)
	}
	if options.Limit != nil && (*options.Limit < 1 || *options.Limit > 100) {
		return newInvalidOptionError(QueryOptionLimit)
	}
	if !validOptionalString(options.Filter) {
		return newInvalidOptionError(QueryOptionFilter)
	}
	if !validOptionalString(options.OrderBy) {
		return newInvalidOptionError(QueryOptionOrderBy)
	}
	if _, err := mergeProjection(options.Select, nil); err != nil {
		return newInvalidOptionError(QueryOptionSelect)
	}
	if _, err := mergeProjection(options.Expand, nil); err != nil {
		return newInvalidOptionError(QueryOptionExpand)
	}
	return nil
}

// Values returns a copy of the query sent to the Nutanix operation.
func (q Query) Values() url.Values {
	return cloneValues(q.wire)
}

// IdentityValues returns caller inputs before mandatory projection fields are added.
func (q Query) IdentityValues() url.Values {
	return cloneValues(q.identity)
}

func newInvalidOptionError(option QueryOption) error {
	return &InvalidOptionError{option: option}
}

func validOptionalString(value *string) bool {
	if value == nil {
		return true
	}
	return validQueryValue(*value)
}

func validQueryValue(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	for index := range len(value) {
		if (value[index] < 0x20 && value[index] != '\t') || value[index] == 0x7f {
			return false
		}
	}
	return true
}

func mergeProjection(caller *string, required []string) (string, error) {
	values := make(map[string]struct{}, len(required))
	for _, value := range required {
		if !validProjectionToken(value) {
			return "", ErrInvalidListQuery
		}
		values[value] = struct{}{}
	}
	if caller != nil {
		if !validQueryValue(*caller) {
			return "", ErrInvalidListQuery
		}
		for token := range strings.SplitSeq(*caller, ",") {
			if !validProjectionToken(token) {
				return "", ErrInvalidListQuery
			}
			values[token] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(values))
	for value := range values {
		ordered = append(ordered, value)
	}
	slices.Sort(ordered)
	return strings.Join(ordered, ","), nil
}

func validProjectionToken(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

func addInteger(wire url.Values, identity url.Values, name string, value *int64) {
	if value == nil {
		return
	}
	encoded := strconv.FormatInt(*value, 10)
	wire.Set(name, encoded)
	identity.Set(name, encoded)
}

func addString(wire url.Values, identity url.Values, name string, value *string) {
	if value == nil {
		return
	}
	wire.Set(name, *value)
	identity.Set(name, *value)
}

func addIdentityString(identity url.Values, name string, value *string) {
	if value != nil {
		identity.Set(name, *value)
	}
}

func cloneValues(source url.Values) url.Values {
	cloned := make(url.Values, len(source))
	for key, values := range source {
		cloned[key] = slices.Clone(values)
	}
	return cloned
}

// Package listquery implements the shared Terraform boundary for OData list queries.
package listquery

import (
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
)

// Values is the common Terraform representation of a Nutanix OData list query.
type Values struct {
	Page    types.Int64
	Limit   types.Int64
	Filter  types.String
	OrderBy types.String
	Select  types.String
	Expand  types.String
}

// DiagnosticText keeps product-specific wording at the service boundary.
type DiagnosticText struct {
	Title  string
	Detail string
}

// QueryAttributes returns the shared optional OData query schema.
func QueryAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"page":     PageAttribute(),
		"limit":    LimitAttribute(),
		"filter":   StringAttribute("OData filter expression."),
		"order_by": StringAttribute("OData order-by expression."),
		"select": StringAttribute(
			"Comma-separated simple properties to request in addition to required state fields.",
		),
	}
}

// CommonEntityAttributes returns the shared computed identity and audit fields.
func CommonEntityAttributes() map[string]schema.Attribute {
	result := make(map[string]schema.Attribute, 7)
	for _, name := range []string{
		"ext_id", "tenant_id", "display_name", "description", "client_name", "created_time", "last_updated_time",
	} {
		result[name] = ComputedString()
	}
	return result
}

// ExtendAttributes returns a copy of base with product-specific fields added.
func ExtendAttributes(base map[string]schema.Attribute, extra map[string]schema.Attribute) map[string]schema.Attribute {
	result := make(map[string]schema.Attribute, len(base)+len(extra))
	for name, attribute := range base {
		result[name] = attribute
	}
	for name, attribute := range extra {
		result[name] = attribute
	}
	return result
}

// ComputedString returns a computed Terraform string attribute.
func ComputedString() schema.StringAttribute { return schema.StringAttribute{Computed: true} }

// ComputedInt64 returns a computed Terraform integer attribute.
func ComputedInt64() schema.Int64Attribute { return schema.Int64Attribute{Computed: true} }

// ComputedBool returns a computed Terraform boolean attribute.
func ComputedBool() schema.BoolAttribute { return schema.BoolAttribute{Computed: true} }

// ComputedList returns a computed Terraform list attribute.
func ComputedList(elementType attr.Type) schema.ListAttribute {
	return schema.ListAttribute{Computed: true, ElementType: elementType}
}

// ComputedID returns the shared computed query identity attribute.
func ComputedID() schema.StringAttribute {
	return schema.StringAttribute{
		Computed:    true,
		Description: "Deterministic identity of the caller-supplied list query.",
	}
}

// PageAttribute returns the shared zero-based page schema.
func PageAttribute() schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional:    true,
		Description: "Zero-based result page. Omitted values use the server default.",
		Validators:  []validator.Int64{int64validator.Between(0, 1<<31-1)},
	}
}

// LimitAttribute returns the shared bounded result-limit schema.
func LimitAttribute() schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional:    true,
		Description: "Maximum records returned, from 1 through 100.",
		Validators:  []validator.Int64{int64validator.Between(1, 100)},
	}
}

// StringAttribute returns a non-empty optional OData string attribute.
func StringAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:    true,
		Description: description,
		Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
	}
}

// Options validates known Terraform values and builds neutral OData options.
func Options(values Values, diagnostic DiagnosticText) (odata.ListOptions, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	checkKnown(&diagnostics, "page", values.Page.IsUnknown(), diagnostic)
	checkKnown(&diagnostics, "limit", values.Limit.IsUnknown(), diagnostic)
	checkKnown(&diagnostics, "filter", values.Filter.IsUnknown(), diagnostic)
	checkKnown(&diagnostics, "order_by", values.OrderBy.IsUnknown(), diagnostic)
	checkKnown(&diagnostics, "select", values.Select.IsUnknown(), diagnostic)
	checkKnown(&diagnostics, "expand", values.Expand.IsUnknown(), diagnostic)
	if diagnostics.HasError() {
		return odata.ListOptions{}, diagnostics
	}
	options := odata.ListOptions{
		Page:    int64Pointer(values.Page),
		Limit:   int64Pointer(values.Limit),
		Filter:  stringPointer(values.Filter),
		OrderBy: stringPointer(values.OrderBy),
		Select:  stringPointer(values.Select),
		Expand:  stringPointer(values.Expand),
	}
	if err := odata.ValidateOptions(options); err != nil {
		addValidationError(&diagnostics, err, diagnostic)
		return odata.ListOptions{}, diagnostics
	}
	return options, diagnostics
}

func checkKnown(
	diagnostics *diag.Diagnostics,
	name string,
	unknown bool,
	diagnostic DiagnosticText,
) {
	if !unknown {
		return
	}
	diagnostics.AddAttributeError(path.Root(name), diagnostic.Title, diagnostic.Detail)
}

func addValidationError(
	diagnostics *diag.Diagnostics,
	err error,
	diagnostic DiagnosticText,
) {
	var invalidOption *odata.InvalidOptionError
	if errors.As(err, &invalidOption) {
		if name, ok := attributeName(invalidOption.Option()); ok {
			diagnostics.AddAttributeError(path.Root(name), diagnostic.Title, diagnostic.Detail)
			return
		}
	}
	diagnostics.AddError(diagnostic.Title, diagnostic.Detail)
}

func attributeName(option odata.QueryOption) (string, bool) {
	switch option {
	case odata.QueryOptionPage:
		return "page", true
	case odata.QueryOptionLimit:
		return "limit", true
	case odata.QueryOptionFilter:
		return "filter", true
	case odata.QueryOptionOrderBy:
		return "order_by", true
	case odata.QueryOptionSelect:
		return "select", true
	case odata.QueryOptionExpand:
		return "expand", true
	default:
		return "", false
	}
}

func int64Pointer(value types.Int64) *int64 {
	if value.IsNull() {
		return nil
	}
	result := value.ValueInt64()
	return &result
}

func stringPointer(value types.String) *string {
	if value.IsNull() {
		return nil
	}
	result := value.ValueString()
	return &result
}

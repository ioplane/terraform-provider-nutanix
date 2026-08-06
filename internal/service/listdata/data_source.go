// Package listdata implements the shared Framework lifecycle for read-only list data sources.
package listdata

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/listquery"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/queryid"
)

// ListFunc is a namespace-owned list operation exposed to a Terraform data source.
type ListFunc[Entity any] func(context.Context, odata.ListOptions) ([]Entity, url.Values, error)

// Descriptor defines the product-specific parts of a read-only list data source.
type Descriptor[Entity any, State any] struct {
	TypeName            string
	IdentityTypeName    string
	Schema              schema.Schema
	Reader              func(any) (ListFunc[Entity], bool)
	Query               func(State) listquery.Values
	SetState            func(context.Context, *State, string, []Entity) diag.Diagnostics
	ConfigureTitle      string
	ConfigureDetail     string
	MissingReaderTitle  string
	MissingReaderDetail string
	ReadErrorTitle      string
	ReadErrorDetail     string
}

// DataSource owns the common Framework configuration, read, identity, and state flow.
type DataSource[Entity any, State any] struct {
	descriptor Descriptor[Entity, State]
	reader     ListFunc[Entity]
}

// New returns a configured generic read-only list data source.
func New[Entity any, State any](descriptor Descriptor[Entity, State]) datasource.DataSource {
	return &DataSource[Entity, State]{descriptor: descriptor}
}

var _ datasource.DataSource = (*DataSource[struct{}, struct{}])(nil)

func (d *DataSource[Entity, State]) Metadata(
	_ context.Context,
	request datasource.MetadataRequest,
	response *datasource.MetadataResponse,
) {
	response.TypeName = request.ProviderTypeName + "_" + d.descriptor.TypeName
}

func (d *DataSource[Entity, State]) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	response *datasource.SchemaResponse,
) {
	response.Schema = d.descriptor.Schema
}

func (d *DataSource[Entity, State]) Configure(
	_ context.Context,
	request datasource.ConfigureRequest,
	response *datasource.ConfigureResponse,
) {
	if request.ProviderData == nil {
		return
	}
	reader, ok := d.descriptor.Reader(request.ProviderData)
	if !ok {
		response.Diagnostics.AddError(
			d.descriptor.ConfigureTitle,
			d.descriptor.ConfigureDetail,
		)
		return
	}
	d.reader = reader
	if d.reader == nil {
		response.Diagnostics.AddError(
			d.descriptor.MissingReaderTitle,
			d.descriptor.MissingReaderDetail,
		)
	}
}

func (d *DataSource[Entity, State]) Read(
	ctx context.Context,
	request datasource.ReadRequest,
	response *datasource.ReadResponse,
) {
	if d.reader == nil {
		response.Diagnostics.AddError(
			d.descriptor.MissingReaderTitle,
			d.descriptor.MissingReaderDetail,
		)
		return
	}
	var state State
	response.Diagnostics.Append(request.Config.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	options, diagnostics := listquery.Options(
		d.descriptor.Query(state),
		listquery.DiagnosticText{
			Title:  "Invalid Nutanix list query",
			Detail: "The query value must be known and valid before the request can be constructed.",
		},
	)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	entities, identity, err := d.reader(ctx, options)
	if err != nil {
		response.Diagnostics.AddError(d.descriptor.ReadErrorTitle, d.descriptor.ReadErrorDetail)
		return
	}
	id, err := queryid.New(d.descriptor.IdentityTypeName, identity)
	if err != nil {
		response.Diagnostics.AddError(d.descriptor.ReadErrorTitle, d.descriptor.ReadErrorDetail)
		return
	}
	response.Diagnostics.Append(d.descriptor.SetState(ctx, &state, id, entities)...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

package listdata

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func TestConfigureRejectsProviderDataWithWrongType(t *testing.T) {
	dataSource := New(Descriptor[struct{}, struct{}]{
		Reader: func(any) (ListFunc[struct{}], bool) { return nil, false },
	})
	response := &datasource.ConfigureResponse{}
	dataSource.(datasource.DataSourceWithConfigure).Configure(context.Background(), datasource.ConfigureRequest{ProviderData: struct{}{}}, response)
	if !response.Diagnostics.HasError() {
		t.Fatal("Configure() diagnostics = nil, want incompatible provider data error")
	}
}

func TestReadRejectsMissingReader(t *testing.T) {
	dataSource := New(Descriptor[struct{}, struct{}]{
		MissingReaderTitle:  "missing reader",
		MissingReaderDetail: "reader is not configured",
	})
	response := &datasource.ReadResponse{}
	dataSource.Read(context.Background(), datasource.ReadRequest{}, response)
	if !response.Diagnostics.HasError() {
		t.Fatal("Read() diagnostics = nil, want missing reader error")
	}
}

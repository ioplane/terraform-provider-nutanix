// Package storagecontainer implements the nutanix_storage_container managed resource.
package storagecontainer

import (
	"context"
	"errors"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/clustermgmt"
	"github.com/ioplane/terraform-provider-nutanix/internal/task"
)

var storageContainerUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Writer is the Cluster Management storage-container capability consumed by this resource.
type Writer interface {
	CreateStorageContainer(context.Context, string, clustermgmt.StorageContainerSpec) (clustermgmt.StorageContainerAsyncOperation, error)
	GetStorageContainerByID(context.Context, string) (clustermgmt.StorageContainerRead, error)
	UpdateStorageContainerByID(context.Context, string, clustermgmt.StorageContainerSpec, string) (clustermgmt.StorageContainerAsyncOperation, error)
	DeleteStorageContainerByID(context.Context, string, bool) (clustermgmt.StorageContainerAsyncOperation, error)
	WaitStorageContainerTask(context.Context, clustermgmt.StorageContainerAsyncOperation) (task.Snapshot, error)
}

type resourceProviderData interface {
	StorageContainerWriter() Writer
}

type resourceModel struct {
	ID                                   types.String `tfsdk:"id"`
	ExtID                                types.String `tfsdk:"ext_id"`
	ClusterExtID                         types.String `tfsdk:"cluster_ext_id"`
	ContainerExtID                       types.String `tfsdk:"container_ext_id"`
	OwnerExtID                           types.String `tfsdk:"owner_ext_id"`
	Name                                 types.String `tfsdk:"name"`
	StoragePoolExtID                     types.String `tfsdk:"storage_pool_ext_id"`
	IsMarkedForRemoval                   types.Bool   `tfsdk:"is_marked_for_removal"`
	MaxCapacityBytes                     types.Int64  `tfsdk:"max_capacity_bytes"`
	LogicalExplicitReservedCapacityBytes types.Int64  `tfsdk:"logical_explicit_reserved_capacity_bytes"`
	LogicalImplicitReservedCapacityBytes types.Int64  `tfsdk:"logical_implicit_reserved_capacity_bytes"`
	LogicalAdvertisedCapacityBytes       types.Int64  `tfsdk:"logical_advertised_capacity_bytes"`
	ReplicationFactor                    types.Int32  `tfsdk:"replication_factor"`
	IsNFSWhitelistInherited              types.Bool   `tfsdk:"is_nfs_whitelist_inherited"`
	ErasureCode                          types.String `tfsdk:"erasure_code"`
	IsInlineECEnabled                    types.Bool   `tfsdk:"is_inline_ec_enabled"`
	HasHigherECFaultDomainPreference     types.Bool   `tfsdk:"has_higher_ec_fault_domain_preference"`
	ErasureCodeDelaySecs                 types.Int32  `tfsdk:"erasure_code_delay_secs"`
	CacheDeduplication                   types.String `tfsdk:"cache_deduplication"`
	OnDiskDedup                          types.String `tfsdk:"on_disk_dedup"`
	IsCompressionEnabled                 types.Bool   `tfsdk:"is_compression_enabled"`
	CompressionDelaySecs                 types.Int32  `tfsdk:"compression_delay_secs"`
	IsInternal                           types.Bool   `tfsdk:"is_internal"`
	IsSoftwareEncryptionEnabled          types.Bool   `tfsdk:"is_software_encryption_enabled"`
	IsEncrypted                          types.Bool   `tfsdk:"is_encrypted"`
	AffinityHostExtID                    types.String `tfsdk:"affinity_host_ext_id"`
	ClusterName                          types.String `tfsdk:"cluster_name"`
	IsShared                             types.Bool   `tfsdk:"is_shared"`
	ExternalStorageExtID                 types.String `tfsdk:"external_storage_ext_id"`
	IgnoreSmallFiles                     types.Bool   `tfsdk:"ignore_small_files"`
}

type resourceImpl struct{ writer Writer }

var (
	_ resource.Resource                = (*resourceImpl)(nil)
	_ resource.ResourceWithConfigure   = (*resourceImpl)(nil)
	_ resource.ResourceWithImportState = (*resourceImpl)(nil)
)

// NewResource returns the Cluster Management storage-container resource.
func NewResource() resource.Resource { return &resourceImpl{} }

func (*resourceImpl) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_storage_container"
}

func (*resourceImpl) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	clusterReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	sharedReplace := []planmodifier.Bool{boolplanmodifier.RequiresReplace()}
	response.Schema = schema.Schema{
		MarkdownDescription: "Manages a Nutanix Cluster Management v4.2 storage container.",
		Attributes: map[string]schema.Attribute{
			"id":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Terraform identity, equal to storage-container `extId`."},
			"ext_id":                schema.StringAttribute{Computed: true, MarkdownDescription: "Storage-container external UUID."},
			"cluster_ext_id":        schema.StringAttribute{Required: true, PlanModifiers: clusterReplace, MarkdownDescription: "PE cluster UUID sent in the API `X-Cluster-Id` header during create.", Validators: []validator.String{stringvalidator.RegexMatches(storageContainerUUIDPattern, "must be a UUID")}},
			"container_ext_id":      computedUUID("Container external UUID."),
			"owner_ext_id":          computedUUID("Owner external UUID."),
			"name":                  schema.StringAttribute{Required: true, MarkdownDescription: "Unique storage-container name in the target cluster.", Validators: []validator.String{stringvalidator.LengthBetween(1, 75)}},
			"storage_pool_ext_id":   computedUUID("Storage pool external UUID."),
			"is_marked_for_removal": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the container is marked for removal by Nutanix."},
			"max_capacity_bytes":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum physical capacity in bytes."},
			"logical_explicit_reserved_capacity_bytes": mutableInt64("Logical explicitly reserved capacity in bytes."),
			"logical_implicit_reserved_capacity_bytes": schema.Int64Attribute{Computed: true, MarkdownDescription: "Logical implicit reservation in bytes."},
			"logical_advertised_capacity_bytes":        mutableInt64("Logical advertised capacity in bytes."),
			"replication_factor":                       mutableInt32("Storage-container replication factor."),
			"is_nfs_whitelist_inherited":               schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the NFS whitelist is inherited globally."},
			"erasure_code":                             mutableEnum("Erasure coding status: `NONE`, `OFF`, or `ON`.", "NONE", "OFF", "ON"),
			"is_inline_ec_enabled":                     schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether inline erasure coding is enabled."},
			"has_higher_ec_fault_domain_preference":    schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether a higher erasure-code fault domain is preferred."},
			"erasure_code_delay_secs":                  mutableInt32("Erasure-code delay in seconds."),
			"cache_deduplication":                      mutableEnum("Cache deduplication status: `NONE`, `OFF`, or `ON`.", "NONE", "OFF", "ON"),
			"on_disk_dedup":                            mutableEnum("On-disk deduplication status: `NONE`, `OFF`, or `POST_PROCESS`.", "NONE", "OFF", "POST_PROCESS"),
			"is_compression_enabled":                   schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether compression is enabled."},
			"compression_delay_secs":                   mutableInt32("Compression delay in seconds."),
			"is_internal":                              schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether Nutanix manages the container internally."},
			"is_software_encryption_enabled":           schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether software encryption is enabled."},
			"is_encrypted":                             schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the container is encrypted."},
			"affinity_host_ext_id":                     schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Affinity host UUID for RF-1 containers.", Validators: []validator.String{stringvalidator.RegexMatches(storageContainerUUIDPattern, "must be a UUID")}},
			"cluster_name":                             schema.StringAttribute{Computed: true, MarkdownDescription: "Owning cluster name."},
			"is_shared":                                schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: sharedReplace, MarkdownDescription: "Whether the container is shared across registered PE clusters. Immutable after creation."},
			"external_storage_ext_id":                  computedUUID("External storage UUID."),
			"ignore_small_files":                       schema.BoolAttribute{Optional: true, MarkdownDescription: "Pass `ignoreSmallFiles` during delete."},
		},
	}
}

func computedUUID(description string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, MarkdownDescription: description}
}

func mutableInt64(description string) schema.Int64Attribute {
	return schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: description, Validators: []validator.Int64{int64validator.AtLeast(0)}}
}

func mutableInt32(description string) schema.Int32Attribute {
	return schema.Int32Attribute{Optional: true, Computed: true, MarkdownDescription: description, Validators: []validator.Int32{int32validator.AtLeast(0)}}
}

func mutableEnum(description string, values ...string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: description, Validators: []validator.String{stringvalidator.OneOf(values...)}}
}

func (r *resourceImpl) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	configured, ok := request.ProviderData.(resourceProviderData)
	if !ok {
		response.Diagnostics.AddError("Unexpected Storage Container Resource Configure Type", "The provider supplied incompatible data to the storage-container resource.")
		return
	}
	r.writer = configured.StorageContainerWriter()
	if r.writer == nil {
		response.Diagnostics.AddError("Missing Storage Container Writer", "The provider did not configure the storage-container writer.")
	}
}

func (r *resourceImpl) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan resourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || !r.requireWriter(&response.Diagnostics) {
		return
	}
	operation, err := r.writer.CreateStorageContainer(ctx, plan.ClusterExtID.ValueString(), specFromModel(plan))
	if err != nil {
		addMutationError(&response.Diagnostics, "create")
		return
	}
	snapshot, err := r.writer.WaitStorageContainerTask(ctx, operation)
	if err != nil {
		addMutationError(&response.Diagnostics, "wait for create")
		return
	}
	extID, err := clustermgmt.StorageContainerIDFromTask(snapshot)
	if err != nil {
		response.Diagnostics.AddError("Unable to identify created Nutanix storage container", "The completed task did not contain exactly one valid storage-container entity.")
		return
	}
	read, err := r.writer.GetStorageContainerByID(ctx, extID)
	if err != nil {
		addMutationError(&response.Diagnostics, "read created storage container")
		return
	}
	state, err := stateFromRemote(plan, read.StorageContainer)
	if err != nil {
		response.Diagnostics.AddError("Unable to map created Nutanix storage container", "The Cluster Management response did not contain a valid storage-container identity.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state resourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || !r.requireWriter(&response.Diagnostics) {
		return
	}
	read, err := r.writer.GetStorageContainerByID(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, clustermgmt.ErrStorageContainerNotFound) {
			response.State.RemoveResource(ctx)
			return
		}
		addMutationError(&response.Diagnostics, "read")
		return
	}
	state, err = stateFromRemote(state, read.StorageContainer)
	if err != nil {
		response.Diagnostics.AddError("Unable to map Nutanix storage container", "The Cluster Management response did not contain a valid storage-container identity.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan resourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || !r.requireWriter(&response.Diagnostics) {
		return
	}
	current, err := r.writer.GetStorageContainerByID(ctx, plan.ID.ValueString())
	if err != nil {
		addMutationError(&response.Diagnostics, "read storage container before update")
		return
	}
	operation, err := r.writer.UpdateStorageContainerByID(ctx, plan.ID.ValueString(), specFromModel(plan), current.ETag)
	if err != nil {
		addMutationError(&response.Diagnostics, "update")
		return
	}
	if _, err := r.writer.WaitStorageContainerTask(ctx, operation); err != nil {
		addMutationError(&response.Diagnostics, "wait for update")
		return
	}
	updated, err := r.writer.GetStorageContainerByID(ctx, plan.ID.ValueString())
	if err != nil {
		addMutationError(&response.Diagnostics, "read updated storage container")
		return
	}
	state, err := stateFromRemote(plan, updated.StorageContainer)
	if err != nil {
		response.Diagnostics.AddError("Unable to map updated Nutanix storage container", "The Cluster Management response did not contain a valid storage-container identity.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state resourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || !r.requireWriter(&response.Diagnostics) {
		return
	}
	operation, err := r.writer.DeleteStorageContainerByID(ctx, state.ID.ValueString(), knownBool(state.IgnoreSmallFiles))
	if err != nil {
		if errors.Is(err, clustermgmt.ErrStorageContainerNotFound) {
			return
		}
		addMutationError(&response.Diagnostics, "delete")
		return
	}
	if _, err := r.writer.WaitStorageContainerTask(ctx, operation); err != nil && !errors.Is(err, clustermgmt.ErrStorageContainerNotFound) {
		addMutationError(&response.Diagnostics, "wait for delete")
	}
}

func (r *resourceImpl) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
}

func (r *resourceImpl) requireWriter(diagnostics *diag.Diagnostics) bool {
	if r.writer != nil {
		return true
	}
	diagnostics.AddError("Missing Storage Container Writer", "The provider did not configure the storage-container writer.")
	return false
}

func specFromModel(model resourceModel) clustermgmt.StorageContainerSpec {
	return clustermgmt.StorageContainerSpec{
		Name:                                 model.Name.ValueString(),
		LogicalExplicitReservedCapacityBytes: knownInt64Pointer(model.LogicalExplicitReservedCapacityBytes),
		LogicalAdvertisedCapacityBytes:       knownInt64Pointer(model.LogicalAdvertisedCapacityBytes),
		ReplicationFactor:                    knownInt32Pointer(model.ReplicationFactor),
		ErasureCode:                          knownStringPointer(model.ErasureCode),
		IsInlineECEnabled:                    knownBoolPointer(model.IsInlineECEnabled),
		HasHigherECFaultDomainPreference:     knownBoolPointer(model.HasHigherECFaultDomainPreference),
		ErasureCodeDelaySecs:                 knownInt32Pointer(model.ErasureCodeDelaySecs),
		CacheDeduplication:                   knownStringPointer(model.CacheDeduplication),
		OnDiskDedup:                          knownStringPointer(model.OnDiskDedup),
		IsCompressionEnabled:                 knownBoolPointer(model.IsCompressionEnabled),
		CompressionDelaySecs:                 knownInt32Pointer(model.CompressionDelaySecs),
		IsSoftwareEncryptionEnabled:          knownBoolPointer(model.IsSoftwareEncryptionEnabled),
		AffinityHostExtID:                    knownStringPointer(model.AffinityHostExtID),
		IsShared:                             knownBoolPointer(model.IsShared),
	}
}

func stateFromRemote(prior resourceModel, container clustermgmt.StorageContainer) (resourceModel, error) {
	if container.ExtID == nil {
		return resourceModel{}, clustermgmt.ErrInvalidStorageContainer
	}
	prior.ID = types.StringPointerValue(container.ExtID)
	prior.ExtID = types.StringPointerValue(container.ExtID)
	prior.ClusterExtID = types.StringPointerValue(container.ClusterExtID)
	prior.ContainerExtID = types.StringPointerValue(container.ContainerExtID)
	prior.OwnerExtID = types.StringPointerValue(container.OwnerExtID)
	prior.Name = types.StringPointerValue(container.Name)
	prior.StoragePoolExtID = types.StringPointerValue(container.StoragePoolExtID)
	prior.IsMarkedForRemoval = types.BoolPointerValue(container.IsMarkedForRemoval)
	prior.MaxCapacityBytes = types.Int64PointerValue(container.MaxCapacityBytes)
	prior.LogicalExplicitReservedCapacityBytes = types.Int64PointerValue(container.LogicalExplicitReservedCapacityBytes)
	prior.LogicalImplicitReservedCapacityBytes = types.Int64PointerValue(container.LogicalImplicitReservedCapacityBytes)
	prior.LogicalAdvertisedCapacityBytes = types.Int64PointerValue(container.LogicalAdvertisedCapacityBytes)
	prior.ReplicationFactor = int32PointerValue(container.ReplicationFactor)
	prior.IsNFSWhitelistInherited = types.BoolPointerValue(container.IsNFSWhitelistInherited)
	prior.ErasureCode = types.StringPointerValue(container.ErasureCode)
	prior.IsInlineECEnabled = types.BoolPointerValue(container.IsInlineECEnabled)
	prior.HasHigherECFaultDomainPreference = types.BoolPointerValue(container.HasHigherECFaultDomainPreference)
	prior.ErasureCodeDelaySecs = int32PointerValue(container.ErasureCodeDelaySecs)
	prior.CacheDeduplication = types.StringPointerValue(container.CacheDeduplication)
	prior.OnDiskDedup = types.StringPointerValue(container.OnDiskDedup)
	prior.IsCompressionEnabled = types.BoolPointerValue(container.IsCompressionEnabled)
	prior.CompressionDelaySecs = int32PointerValue(container.CompressionDelaySecs)
	prior.IsInternal = types.BoolPointerValue(container.IsInternal)
	prior.IsSoftwareEncryptionEnabled = types.BoolPointerValue(container.IsSoftwareEncryptionEnabled)
	prior.IsEncrypted = types.BoolPointerValue(container.IsEncrypted)
	prior.AffinityHostExtID = types.StringPointerValue(container.AffinityHostExtID)
	prior.ClusterName = types.StringPointerValue(container.ClusterName)
	prior.IsShared = types.BoolPointerValue(container.IsShared)
	prior.ExternalStorageExtID = types.StringPointerValue(container.ExternalStorageExtID)
	return prior, nil
}

func knownStringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	text := value.ValueString()
	return &text
}

func knownBoolPointer(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	selected := value.ValueBool()
	return &selected
}

func knownInt64Pointer(value types.Int64) *int64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	number := value.ValueInt64()
	return &number
}

func knownInt32Pointer(value types.Int32) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	number := value.ValueInt32()
	return &number
}

func int32PointerValue(value *int32) types.Int32 {
	if value == nil {
		return types.Int32Null()
	}
	return types.Int32Value(*value)
}

func knownBool(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueBool()
}

func addMutationError(diagnostics *diag.Diagnostics, operation string) {
	diagnostics.AddError("Nutanix storage-container operation failed", "The storage-container "+operation+" operation could not be completed.")
}

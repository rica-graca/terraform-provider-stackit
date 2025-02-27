package dns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/stackitcloud/stackit-sdk-go/core/config"
	"github.com/stackitcloud/stackit-sdk-go/services/dns"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/conversion"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/core"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/validate"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &recordSetResource{}
	_ resource.ResourceWithConfigure   = &recordSetResource{}
	_ resource.ResourceWithImportState = &recordSetResource{}
)

type Model struct {
	Id          types.String `tfsdk:"id"` // needed by TF
	RecordSetId types.String `tfsdk:"record_set_id"`
	ZoneId      types.String `tfsdk:"zone_id"`
	ProjectId   types.String `tfsdk:"project_id"`
	Active      types.Bool   `tfsdk:"active"`
	Comment     types.String `tfsdk:"comment"`
	Name        types.String `tfsdk:"name"`
	Records     types.List   `tfsdk:"records"`
	TTL         types.Int64  `tfsdk:"ttl"`
	Type        types.String `tfsdk:"type"`
	Error       types.String `tfsdk:"error"`
	State       types.String `tfsdk:"state"`
}

// NewRecordSetResource is a helper function to simplify the provider implementation.
func NewRecordSetResource() resource.Resource {
	return &recordSetResource{}
}

// recordSetResource is the resource implementation.
type recordSetResource struct {
	client *dns.APIClient
}

// Metadata returns the resource type name.
func (r *recordSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record_set"
}

// Configure adds the provider configured client to the resource.
func (r *recordSetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(core.ProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected stackit.ProviderData, got %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}

	var apiClient *dns.APIClient
	var err error
	if providerData.DnsCustomEndpoint != "" {
		apiClient, err = dns.NewAPIClient(
			config.WithCustomAuth(providerData.RoundTripper),
			config.WithEndpoint(providerData.DnsCustomEndpoint),
		)
	} else {
		apiClient, err = dns.NewAPIClient(
			config.WithCustomAuth(providerData.RoundTripper),
		)
	}

	if err != nil {
		resp.Diagnostics.AddError("Could not Configure API Client", err.Error())
		return
	}

	tflog.Debug(ctx, "DNS record set client configured")
	r.client = apiClient
}

// Schema defines the schema for the resource.
func (r *recordSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "DNS Record Set Resource schema.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Terraform's internal resource ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "STACKIT project ID to which the dns record set is associated.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validate.UUID(),
					validate.NoSeparator(),
				},
			},
			"zone_id": schema.StringAttribute{
				Description: "The zone ID to which is dns record set is associated.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validate.UUID(),
					validate.NoSeparator(),
				},
			},
			"record_set_id": schema.StringAttribute{
				Description: "The rr set id.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					validate.UUID(),
					validate.NoSeparator(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the record which should be a valid domain according to rfc1035 Section 2.3.4. E.g. `example.com`",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.LengthAtMost(63),
				},
			},
			"records": schema.ListAttribute{
				Description: "Records.",
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.UniqueValues(),
					listvalidator.ValueStringsAre(validate.IP()),
				},
			},
			"ttl": schema.Int64Attribute{
				Description: "Time to live. E.g. 3600",
				Optional:    true,
				Computed:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(30),
					int64validator.AtMost(99999999),
				},
			},
			"type": schema.StringAttribute{
				Description: "The record set type. E.g. `A` or `CNAME`",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"active": schema.BoolAttribute{
				Description: "Specifies if the record set is active or not.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"comment": schema.StringAttribute{
				Description: "Comment.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(255),
				},
			},
			"error": schema.StringAttribute{
				Description: "Error shows error in case create/update/delete failed.",
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(2000),
				},
			},
			"state": schema.StringAttribute{
				Description: "Record set state.",
				Computed:    true,
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *recordSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) { // nolint:gocritic // function signature required by Terraform
	// Retrieve values from plan
	var model Model
	diags := req.Plan.Get(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectId := model.ProjectId.ValueString()
	zoneId := model.ZoneId.ValueString()
	ctx = tflog.SetField(ctx, "project_id", projectId)
	ctx = tflog.SetField(ctx, "zone_id", zoneId)

	// Generate API request body from model
	payload, err := toCreatePayload(&model)
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error creating recordset", fmt.Sprintf("Creating API payload: %v", err))
		return
	}
	// Create new recordset
	recordSetResp, err := r.client.CreateRecordSet(ctx, projectId, zoneId).CreateRecordSetPayload(*payload).Execute()
	if err != nil || recordSetResp.Rrset == nil || recordSetResp.Rrset.Id == nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error creating recordset", fmt.Sprintf("Calling API: %v", err))
		return
	}
	ctx = tflog.SetField(ctx, "record_set_id", *recordSetResp.Rrset.Id)

	wr, err := dns.CreateRecordSetWaitHandler(ctx, r.client, projectId, zoneId, *recordSetResp.Rrset.Id).SetTimeout(1 * time.Minute).WaitWithContext(ctx)
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error creating recordset", fmt.Sprintf("Instance creation waiting: %v", err))
		return
	}
	got, ok := wr.(*dns.RecordSetResponse)
	if !ok {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error creating recordset", fmt.Sprintf("Wait result conversion, got %+v", got))
		return
	}

	// Map response body to schema and populate Computed attribute values
	err = mapFields(got, &model)
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error mapping fields", err.Error())
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
	tflog.Info(ctx, "DNS record set created")
}

// Read refreshes the Terraform state with the latest data.
func (r *recordSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) { // nolint:gocritic // function signature required by Terraform
	var model Model
	diags := req.State.Get(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectId := model.ProjectId.ValueString()
	zoneId := model.ZoneId.ValueString()
	recordSetId := model.RecordSetId.ValueString()
	ctx = tflog.SetField(ctx, "project_id", projectId)
	ctx = tflog.SetField(ctx, "zone_id", zoneId)
	ctx = tflog.SetField(ctx, "record_set_id", recordSetId)

	recordSetResp, err := r.client.GetRecordSet(ctx, projectId, zoneId, recordSetId).Execute()
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error reading zones", err.Error())
		return
	}

	// Map response body to schema and populate Computed attribute values
	err = mapFields(recordSetResp, &model)
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error mapping fields", err.Error())
		return
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
	tflog.Info(ctx, "DNS record set read")
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *recordSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) { // nolint:gocritic // function signature required by Terraform
	// Retrieve values from plan
	var model Model
	diags := req.Plan.Get(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectId := model.ProjectId.ValueString()
	zoneId := model.ZoneId.ValueString()
	recordSetId := model.RecordSetId.ValueString()
	ctx = tflog.SetField(ctx, "project_id", projectId)
	ctx = tflog.SetField(ctx, "zone_id", zoneId)
	ctx = tflog.SetField(ctx, "record_set_id", recordSetId)

	// Generate API request body from model
	payload, err := toUpdatePayload(&model)
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error updating recordset", fmt.Sprintf("Could not create API payload: %v", err))
		return
	}
	// Update recordset
	_, err = r.client.UpdateRecordSet(ctx, projectId, zoneId, recordSetId).UpdateRecordSetPayload(*payload).Execute()
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error updating recordset", err.Error())
		return
	}
	wr, err := dns.UpdateRecordSetWaitHandler(ctx, r.client, projectId, zoneId, recordSetId).SetTimeout(1 * time.Minute).WaitWithContext(ctx)
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error updating recordset", fmt.Sprintf("Instance update waiting: %v", err))
		return
	}
	got, ok := wr.(*dns.RecordSetResponse)
	if !ok {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error updating recordset", fmt.Sprintf("Wait result conversion, got %+v", got))
		return
	}

	// Fetch updated record set
	recordSetResp, err := r.client.GetRecordSet(ctx, projectId, zoneId, recordSetId).Execute()
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error reading updated data", err.Error())
		return
	}
	err = mapFields(recordSetResp, &model)
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error mapping fields in update", err.Error())
		return
	}
	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
	tflog.Info(ctx, "DNS record set updated")
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *recordSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) { // nolint:gocritic // function signature required by Terraform
	// Retrieve values from plan
	var model Model
	diags := req.State.Get(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectId := model.ProjectId.ValueString()
	zoneId := model.ZoneId.ValueString()
	recordSetId := model.RecordSetId.ValueString()
	ctx = tflog.SetField(ctx, "project_id", projectId)
	ctx = tflog.SetField(ctx, "zone_id", zoneId)
	ctx = tflog.SetField(ctx, "record_set_id", recordSetId)

	// Delete existing record set
	_, err := r.client.DeleteRecordSet(ctx, projectId, zoneId, recordSetId).Execute()
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error deleting recordset", err.Error())
	}
	_, err = dns.DeleteRecordSetWaitHandler(ctx, r.client, projectId, zoneId, recordSetId).SetTimeout(1 * time.Minute).WaitWithContext(ctx)
	if err != nil {
		core.LogAndAddError(ctx, &resp.Diagnostics, "Error deleting record set", fmt.Sprintf("Instance deletion waiting: %v", err))
		return
	}
	tflog.Info(ctx, "DNS record set deleted")
}

// ImportState imports a resource into the Terraform state on success.
// The expected format of the resource import identifier is: project_id,zone_id,record_set_id
func (r *recordSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, core.Separator)
	if len(idParts) != 3 || idParts[0] == "" || idParts[1] == "" || idParts[2] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format [project_id],[zone_id],[record_set_id], got %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_id"), idParts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("record_set_id"), idParts[2])...)
	tflog.Info(ctx, "DNS record set state imported")
}

func mapFields(recordSetResp *dns.RecordSetResponse, model *Model) error {
	if recordSetResp == nil || recordSetResp.Rrset == nil {
		return fmt.Errorf("response input is nil")
	}
	if model == nil {
		return fmt.Errorf("model input is nil")
	}
	recordSet := recordSetResp.Rrset

	var recordSetId string
	if model.RecordSetId.ValueString() != "" {
		recordSetId = model.RecordSetId.ValueString()
	} else if recordSet.Id != nil {
		recordSetId = *recordSet.Id
	} else {
		return fmt.Errorf("record set id not present")
	}

	if recordSet.Records == nil {
		model.Records = types.ListNull(types.StringType)
	} else {
		records := []attr.Value{}
		for _, record := range *recordSet.Records {
			records = append(records, types.StringPointerValue(record.Content))
		}
		recordsList, diags := types.ListValue(types.StringType, records)
		if diags.HasError() {
			return fmt.Errorf("failed to map records: %w", core.DiagsToError(diags))
		}
		model.Records = recordsList
	}
	idParts := []string{
		model.ProjectId.ValueString(),
		model.ZoneId.ValueString(),
		recordSetId,
	}
	model.Id = types.StringValue(
		strings.Join(idParts, core.Separator),
	)
	model.RecordSetId = types.StringPointerValue(recordSet.Id)
	model.Active = types.BoolPointerValue(recordSet.Active)
	model.Comment = types.StringPointerValue(recordSet.Comment)
	model.Error = types.StringPointerValue(recordSet.Error)
	model.Name = types.StringPointerValue(recordSet.Name)
	model.State = types.StringPointerValue(recordSet.State)
	model.TTL = conversion.ToTypeInt64(recordSet.Ttl)
	model.Type = types.StringPointerValue(recordSet.Type)
	return nil
}

func toCreatePayload(model *Model) (*dns.CreateRecordSetPayload, error) {
	if model == nil {
		return nil, fmt.Errorf("nil model")
	}

	records := []dns.RecordPayload{}
	for i, record := range model.Records.Elements() {
		recordString, ok := record.(types.String)
		if !ok {
			return nil, fmt.Errorf("expected record at index %d to be of type %T, got %T", i, types.String{}, record)
		}
		records = append(records, dns.RecordPayload{
			Content: recordString.ValueStringPointer(),
		})
	}

	return &dns.CreateRecordSetPayload{
		Comment: model.Comment.ValueStringPointer(),
		Name:    model.Name.ValueStringPointer(),
		Records: &records,
		Ttl:     conversion.ToPtrInt32(model.TTL),
		Type:    model.Type.ValueStringPointer(),
	}, nil
}

func toUpdatePayload(model *Model) (*dns.UpdateRecordSetPayload, error) {
	if model == nil {
		return nil, fmt.Errorf("nil model")
	}

	records := []dns.RecordPayload{}
	for i, record := range model.Records.Elements() {
		recordString, ok := record.(types.String)
		if !ok {
			return nil, fmt.Errorf("expected record at index %d to be of type %T, got %T", i, types.String{}, record)
		}
		records = append(records, dns.RecordPayload{
			Content: recordString.ValueStringPointer(),
		})
	}

	return &dns.UpdateRecordSetPayload{
		Comment: model.Comment.ValueStringPointer(),
		Name:    model.Name.ValueStringPointer(),
		Records: &records,
		Ttl:     conversion.ToPtrInt32(model.TTL),
	}, nil
}

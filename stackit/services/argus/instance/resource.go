package argus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackitcloud/stackit-sdk-go/core/config"
	"github.com/stackitcloud/stackit-sdk-go/services/argus"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/core"
	"github.com/stackitcloud/terraform-provider-stackit/stackit/validate"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &instanceResource{}
	_ resource.ResourceWithConfigure   = &instanceResource{}
	_ resource.ResourceWithImportState = &instanceResource{}
)

type Model struct {
	Id                                 types.String `tfsdk:"id"` // needed by TF
	ProjectId                          types.String `tfsdk:"project_id"`
	InstanceId                         types.String `tfsdk:"instance_id"`
	Name                               types.String `tfsdk:"name"`
	PlanName                           types.String `tfsdk:"plan_name"`
	PlanId                             types.String `tfsdk:"plan_id"`
	Parameters                         types.Map    `tfsdk:"parameters"`
	DashboardURL                       types.String `tfsdk:"dashboard_url"`
	IsUpdatable                        types.Bool   `tfsdk:"is_updatable"`
	GrafanaURL                         types.String `tfsdk:"grafana_url"`
	GrafanaPublicReadAccess            types.Bool   `tfsdk:"grafana_public_read_access"`
	GrafanaInitialAdminPassword        types.String `tfsdk:"grafana_initial_admin_password"`
	GrafanaInitialAdminUser            types.String `tfsdk:"grafana_initial_admin_user"`
	MetricsRetentionDays               types.Int64  `tfsdk:"metrics_retention_days"`
	MetricsRetentionDays5mDownsampling types.Int64  `tfsdk:"metrics_retention_days_5m_downsampling"`
	MetricsRetentionDays1hDownsampling types.Int64  `tfsdk:"metrics_retention_days_1h_downsampling"`
	MetricsURL                         types.String `tfsdk:"metrics_url"`
	MetricsPushURL                     types.String `tfsdk:"metrics_push_url"`
	TargetsURL                         types.String `tfsdk:"targets_url"`
	AlertingURL                        types.String `tfsdk:"alerting_url"`
	LogsURL                            types.String `tfsdk:"logs_url"`
	LogsPushURL                        types.String `tfsdk:"logs_push_url"`
	JaegerTracesURL                    types.String `tfsdk:"jaeger_traces_url"`
	JaegerUIURL                        types.String `tfsdk:"jaeger_ui_url"`
	OtlpTracesURL                      types.String `tfsdk:"otlp_traces_url"`
	ZipkinSpansURL                     types.String `tfsdk:"zipkin_spans_url"`
}

// NewInstanceResource is a helper function to simplify the provider implementation.
func NewInstanceResource() resource.Resource {
	return &instanceResource{}
}

// instanceResource is the resource implementation.
type instanceResource struct {
	client *argus.APIClient
}

// Metadata returns the resource type name.
func (r *instanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_argus_instance"
}

// Configure adds the provider configured client to the resource.
func (r *instanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(core.ProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected stackit.ProviderData, got %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}

	var apiClient *argus.APIClient
	var err error
	if providerData.ArgusCustomEndpoint != "" {
		apiClient, err = argus.NewAPIClient(
			config.WithCustomAuth(providerData.RoundTripper),
			config.WithEndpoint(providerData.ArgusCustomEndpoint),
		)
	} else {
		apiClient, err = argus.NewAPIClient(
			config.WithCustomAuth(providerData.RoundTripper),
			config.WithRegion(providerData.Region),
		)
	}

	if err != nil {
		resp.Diagnostics.AddError("Could not Configure API Client", err.Error())
		return
	}
	r.client = apiClient
}

// Schema defines the schema for the resource.
func (r *instanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Terraform's internal resource ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "STACKIT project ID to which the instance is associated.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					validate.UUID(),
					validate.NoSeparator(),
				},
			},
			"instance_id": schema.StringAttribute{
				Description: "The Argus instance ID.",
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
				Description: "The name of the Argus instance.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.LengthAtMost(200),
				},
			},
			"plan_name": schema.StringAttribute{
				Description: "Specifies the Argus plan. E.g. `Monitoring-Medium-EU01`.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.LengthAtMost(200),
				},
			},
			"plan_id": schema.StringAttribute{
				Description: "The Argus plan ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					validate.UUID(),
				},
			},
			"parameters": schema.MapAttribute{
				Description: "Additional parameters.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"dashboard_url": schema.StringAttribute{
				Description: "Specifies Argus instance dashboard URL.",
				Computed:    true,
			},
			"is_updatable": schema.BoolAttribute{
				Description: "Specifies if the instance can be updated.",
				Computed:    true,
			},
			"grafana_public_read_access": schema.BoolAttribute{
				Description: "If true, anyone can access Grafana dashboards without logging in.",
				Computed:    true,
			},
			"grafana_url": schema.StringAttribute{
				Description: "Specifies Grafana URL.",
				Computed:    true,
			},
			"grafana_initial_admin_user": schema.StringAttribute{
				Description: "Specifies an initial Grafana admin username.",
				Computed:    true,
			},
			"grafana_initial_admin_password": schema.StringAttribute{
				Description: "Specifies an initial Grafana admin password.",
				Computed:    true,
				Sensitive:   true,
			},
			"metrics_retention_days": schema.Int64Attribute{
				Description: "Specifies for how many days the raw metrics are kept.",
				Computed:    true,
			},
			"metrics_retention_days_5m_downsampling": schema.Int64Attribute{
				Description: "Specifies for how many days the 5m downsampled metrics are kept. must be less than the value of the general retention. Default is set to `0` (disabled).",
				Computed:    true,
			},
			"metrics_retention_days_1h_downsampling": schema.Int64Attribute{
				Description: "Specifies for how many days the 1h downsampled metrics are kept. must be less than the value of the 5m downsampling retention. Default is set to `0` (disabled).",
				Computed:    true,
			},
			"metrics_url": schema.StringAttribute{
				Description: "Specifies metrics URL.",
				Computed:    true,
			},
			"metrics_push_url": schema.StringAttribute{
				Description: "Specifies URL for pushing metrics.",
				Computed:    true,
			},
			"targets_url": schema.StringAttribute{
				Description: "Specifies Targets URL.",
				Computed:    true,
			},
			"alerting_url": schema.StringAttribute{
				Description: "Specifies Alerting URL.",
				Computed:    true,
			},
			"logs_url": schema.StringAttribute{
				Description: "Specifies Logs URL.",
				Computed:    true,
			},
			"logs_push_url": schema.StringAttribute{
				Description: "Specifies URL for pushing logs.",
				Computed:    true,
			},
			"jaeger_traces_url": schema.StringAttribute{
				Computed: true,
			},
			"jaeger_ui_url": schema.StringAttribute{
				Computed: true,
			},
			"otlp_traces_url": schema.StringAttribute{
				Computed: true,
			},
			"zipkin_spans_url": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *instanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) { // nolint:gocritic // function signature required by Terraform
	// Retrieve values from plan
	var model Model
	diags := req.Plan.Get(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectId := model.ProjectId.ValueString()

	r.loadPlanId(ctx, &resp.Diagnostics, &model)
	if diags.HasError() {
		core.LogAndAddError(ctx, &diags, "Failed to load argus service plan", "plan "+model.PlanName.ValueString())
		return
	}
	// Generate API request body from model
	payload, err := toCreatePayload(&model)
	if err != nil {
		resp.Diagnostics.AddError("Error creating instance", fmt.Sprintf("Creating API payload: %v", err))
		return
	}
	createResp, err := r.client.CreateInstance(ctx, projectId).CreateInstancePayload(*payload).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating instance", fmt.Sprintf("Calling API: %v", err))
		return
	}
	instanceId := createResp.InstanceId
	if instanceId == nil || *instanceId == "" {
		resp.Diagnostics.AddError("Error creating instance", "API didn't return an instance id")
		return
	}
	wr, err := argus.CreateInstanceWaitHandler(ctx, r.client, *instanceId, projectId).SetTimeout(20 * time.Minute).WaitWithContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error creating instance", fmt.Sprintf("Instance creation waiting: %v", err))
		return
	}
	got, ok := wr.(*argus.InstanceResponse)
	if !ok {
		resp.Diagnostics.AddError("Error creating instance", fmt.Sprintf("Wait result conversion, got %+v", got))
		return
	}

	// Map response body to schema and populate Computed attribute values
	err = mapFields(ctx, got, &model)
	if err != nil {
		resp.Diagnostics.AddError("Error mapping fields", fmt.Sprintf("Project id %s, instance id %s: %v", projectId, *instanceId, err))
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *instanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) { // nolint:gocritic // function signature required by Terraform
	var model Model
	diags := req.State.Get(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectId := model.ProjectId.ValueString()
	instanceId := model.InstanceId.ValueString()

	instanceResp, err := r.client.GetInstance(ctx, instanceId, projectId).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error reading instance", fmt.Sprintf("Project id = %s, instance id = %s: %v", projectId, instanceId, err))
		return
	}

	// Map response body to schema and populate Computed attribute values
	err = mapFields(ctx, instanceResp, &model)
	if err != nil {
		resp.Diagnostics.AddError("Error mapping fields", fmt.Sprintf("Project id %s, instance id %s: %v", projectId, instanceId, err))
		return
	}
	// Set refreshed model
	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *instanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) { // nolint:gocritic // function signature required by Terraform
	// Retrieve values from plan
	var model Model
	diags := req.Plan.Get(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectId := model.ProjectId.ValueString()
	instanceId := model.InstanceId.ValueString()

	r.loadPlanId(ctx, &resp.Diagnostics, &model)
	if diags.HasError() {
		core.LogAndAddError(ctx, &diags, "Failed to load argus service plan", "plan "+model.PlanName.ValueString())
		return
	}

	// Generate API request body from model
	payload, err := toUpdatePayload(&model)
	if err != nil {
		resp.Diagnostics.AddError("Error updating instance", fmt.Sprintf("Could not create API payload: %v", err))
		return
	}
	// Update existing instance
	_, err = r.client.UpdateInstance(ctx, instanceId, projectId).UpdateInstancePayload(*payload).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error updating instance", "project id = "+projectId+", instance Id = "+instanceId+", "+err.Error())
		return
	}
	wr, err := argus.UpdateInstanceWaitHandler(ctx, r.client, instanceId, projectId).SetTimeout(20 * time.Minute).WaitWithContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error updating instance", fmt.Sprintf("Instance update waiting: %v", err))
		return
	}
	got, ok := wr.(*argus.InstanceResponse)
	if !ok {
		resp.Diagnostics.AddError("Error updating instance", fmt.Sprintf("Wait result conversion, got %+v", got))
		return
	}

	err = mapFields(ctx, got, &model)
	if err != nil {
		resp.Diagnostics.AddError("Error mapping fields in update", "project id = "+projectId+", instance Id = "+instanceId+", "+err.Error())
		return
	}
	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *instanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) { // nolint:gocritic // function signature required by Terraform
	// Retrieve values from state
	var model Model
	diags := req.State.Get(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectId := model.ProjectId.ValueString()
	instanceId := model.InstanceId.ValueString()

	// Delete existing instance
	_, err := r.client.DeleteInstance(ctx, instanceId, projectId).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error deleting instance", "project id = "+projectId+", instance Id = "+instanceId+", "+err.Error())
		return
	}
	_, err = argus.DeleteInstanceWaitHandler(ctx, r.client, instanceId, projectId).SetTimeout(10 * time.Minute).WaitWithContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting instance", fmt.Sprintf("Instance deletion waiting: %v", err))
		return
	}
}

// ImportState imports a resource into the Terraform state on success.
// The expected format of the resource import identifier is: project_id,instance_id
func (r *instanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, core.Separator)

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: [project_id],[instance_id]  Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance_id"), idParts[1])...)
}

func mapFields(ctx context.Context, r *argus.InstanceResponse, model *Model) error {
	if r == nil {
		return fmt.Errorf("response input is nil")
	}
	if model == nil {
		return fmt.Errorf("model input is nil")
	}
	var instanceId string
	if model.InstanceId.ValueString() != "" {
		instanceId = model.InstanceId.ValueString()
	} else if r.Id != nil {
		instanceId = *r.Id
	} else {
		return fmt.Errorf("instance id not present")
	}

	idParts := []string{
		model.ProjectId.ValueString(),
		instanceId,
	}
	model.Id = types.StringValue(
		strings.Join(idParts, core.Separator),
	)
	model.InstanceId = types.StringValue(instanceId)
	model.PlanName = types.StringPointerValue(r.PlanName)
	model.PlanId = types.StringPointerValue(r.PlanId)
	model.Name = types.StringPointerValue(r.Name)

	ps := r.Parameters
	if ps == nil {
		model.Parameters = types.MapNull(types.StringType)
	} else {
		params := make(map[string]attr.Value, len(*ps))
		for k, v := range *ps {
			params[k] = types.StringValue(v)
		}
		res, diags := types.MapValueFrom(ctx, types.StringType, params)
		if diags.HasError() {
			return fmt.Errorf("parameter mapping %s", diags.Errors())
		}
		model.Parameters = res
	}

	model.IsUpdatable = types.BoolPointerValue(r.IsUpdatable)
	model.DashboardURL = types.StringPointerValue(r.DashboardUrl)
	if r.Instance != nil {
		i := *r.Instance
		model.GrafanaURL = types.StringPointerValue(i.GrafanaUrl)
		model.GrafanaPublicReadAccess = types.BoolPointerValue(i.GrafanaPublicReadAccess)
		model.GrafanaInitialAdminPassword = types.StringPointerValue(i.GrafanaAdminPassword)
		model.GrafanaInitialAdminUser = types.StringPointerValue(i.GrafanaAdminUser)
		model.MetricsRetentionDays = types.Int64Value(int64(*i.MetricsRetentionTimeRaw))
		model.MetricsRetentionDays5mDownsampling = types.Int64Value(int64(*i.MetricsRetentionTime5m))
		model.MetricsRetentionDays1hDownsampling = types.Int64Value(int64(*i.MetricsRetentionTime1h))
		model.MetricsURL = types.StringPointerValue(i.MetricsUrl)
		model.MetricsPushURL = types.StringPointerValue(i.PushMetricsUrl)
		model.TargetsURL = types.StringPointerValue(i.TargetsUrl)
		model.AlertingURL = types.StringPointerValue(i.AlertingUrl)
		model.LogsURL = types.StringPointerValue(i.LogsUrl)
		model.LogsPushURL = types.StringPointerValue(i.LogsPushUrl)
		model.JaegerTracesURL = types.StringPointerValue(i.JaegerTracesUrl)
		model.JaegerUIURL = types.StringPointerValue(i.JaegerUiUrl)
		model.OtlpTracesURL = types.StringPointerValue(i.OtlpTracesUrl)
		model.ZipkinSpansURL = types.StringPointerValue(i.ZipkinSpansUrl)
	}
	return nil
}

func toCreatePayload(model *Model) (*argus.CreateInstancePayload, error) {
	if model == nil {
		return nil, fmt.Errorf("nil model")
	}
	elements := model.Parameters.Elements()
	pa := make(map[string]interface{}, len(elements))
	for k := range elements {
		pa[k] = elements[k].String()
	}
	return &argus.CreateInstancePayload{
		Name:      model.Name.ValueStringPointer(),
		PlanId:    model.PlanId.ValueStringPointer(),
		Parameter: &pa,
	}, nil
}

func toUpdatePayload(model *Model) (*argus.UpdateInstancePayload, error) {
	if model == nil {
		return nil, fmt.Errorf("nil model")
	}
	elements := model.Parameters.Elements()
	pa := make(map[string]interface{}, len(elements))
	for k, v := range elements {
		pa[k] = v.String()
	}
	return &argus.UpdateInstancePayload{
		Name:      model.Name.ValueStringPointer(),
		PlanId:    model.PlanId.ValueStringPointer(),
		Parameter: &pa,
	}, nil
}

func (r *instanceResource) loadPlanId(ctx context.Context, diags *diag.Diagnostics, model *Model) {
	projectId := model.ProjectId.ValueString()
	res, err := r.client.GetPlans(ctx, projectId).Execute()
	if err != nil {
		diags.AddError("Failed to list argus plans", err.Error())
		return
	}

	planName := model.PlanName.ValueString()
	avl := ""
	plans := *res.Plans
	for i := range plans {
		p := plans[i]
		if p.Name == nil {
			continue
		}
		if strings.EqualFold(*p.Name, planName) && p.PlanId != nil {
			model.PlanId = types.StringPointerValue(p.PlanId)
			break
		}
		avl = fmt.Sprintf("%s\n- %s", avl, *p.Name)
	}
	if model.PlanId.ValueString() == "" {
		diags.AddError("Invalid plan_name", fmt.Sprintf("Couldn't find plan_name '%s', available names are:%s", planName, avl))
		return
	}
}

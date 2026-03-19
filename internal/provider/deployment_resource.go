package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moghtech/terraform-provider-komodo/internal/client"
)

var (
	_ resource.Resource                = &DeploymentResource{}
	_ resource.ResourceWithImportState = &DeploymentResource{}
	_ resource.ResourceWithModifyPlan  = &DeploymentResource{}
)

type DeploymentResource struct {
	client *client.Client
}

type DeploymentResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	ServerID    types.String `tfsdk:"server_id"`
	ImageType   types.String `tfsdk:"image_type"`
	Image       types.String `tfsdk:"image"`
	Network     types.String `tfsdk:"network"`
	RestartMode types.String `tfsdk:"restart_mode"`
	Command     types.String `tfsdk:"command"`
	SendAlerts  types.Bool   `tfsdk:"send_alerts"`
	AutoUpdate  types.Bool   `tfsdk:"auto_update"`
	Ports       types.String `tfsdk:"ports"`
	Volumes     types.String `tfsdk:"volumes"`
	Environment types.String `tfsdk:"environment"`
	Labels      types.String `tfsdk:"labels"`
	ExtraArgs   types.List   `tfsdk:"extra_args"`
	Started     types.Bool   `tfsdk:"started"`
	Deployed    types.Bool   `tfsdk:"deployed"`
	Tags        types.List   `tfsdk:"tags"`
}

func NewDeploymentResource() resource.Resource {
	return &DeploymentResource{}
}

func (r *DeploymentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

func (r *DeploymentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Komodo deployment (a container deployment).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the deployment.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Display name of the deployment.",
				Required:    true,
			},
			"server_id": schema.StringAttribute{
				Description: "The server to deploy on (ID or name).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"image_type": schema.StringAttribute{
				Description: `Image source type: "Image" for an external docker image, "Build" for a Komodo Build.`,
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Image"),
			},
			"image": schema.StringAttribute{
				Description: `Docker image name (e.g. "nginx:latest") when image_type = "Image", or the build ID when image_type = "Build".`,
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"network": schema.StringAttribute{
				Description: "Docker network to connect to.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"restart_mode": schema.StringAttribute{
				Description: "Restart policy (e.g., unless-stopped, always, on-failure, no).",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"command": schema.StringAttribute{
				Description: "Override the container command.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"send_alerts": schema.BoolAttribute{
				Description: "Whether to send deployment state change alerts.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"auto_update": schema.BoolAttribute{
				Description: "Automatically redeploy when newer images are found.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"ports": schema.StringAttribute{
				Description: "Port mappings, newline-separated (e.g., \"8080:80\\n9090:90\").",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"volumes": schema.StringAttribute{
				Description: "Volume mounts, newline-separated (e.g., \"/host/path:/container/path\").",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"environment": schema.StringAttribute{
				Description: "Environment variables, newline-separated KEY=VALUE pairs.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"labels": schema.StringAttribute{
				Description: "Container labels, newline-separated KEY=VALUE pairs.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"extra_args": schema.ListAttribute{
				Description: "Extra arguments for docker run.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"started": schema.BoolAttribute{
				Description: "Whether the deployment container should be running. When true, runs Deploy (docker run). When false, runs DestroyDeployment (docker stop + rm). Intent-based and async.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"deployed": schema.BoolAttribute{
				Description: "When true, triggers a full Deploy (pull image + docker run) after create/update. Intent-based and async.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"tags": schema.ListAttribute{
				Description: "List of tag names to apply to this deployment.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *DeploymentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func deploymentConfigFromModel(ctx context.Context, m *DeploymentResourceModel, diags *resource.CreateResponse) (client.DeploymentConfig, error) {
	imgType := m.ImageType.ValueString()
	if imgType == "" {
		imgType = "Image"
	}
	var imgParams client.DeploymentImageParams
	if imgType == "Build" {
		imgParams.BuildID = m.Image.ValueString()
	} else {
		imgParams.Image = m.Image.ValueString()
	}
	cfg := client.DeploymentConfig{
		ServerID:    m.ServerID.ValueString(),
		Image:       client.DeploymentImage{Type: imgType, Params: imgParams},
		Network:     m.Network.ValueString(),
		RestartMode: m.RestartMode.ValueString(),
		Command:     m.Command.ValueString(),
		Ports:       m.Ports.ValueString(),
		Volumes:     m.Volumes.ValueString(),
		Environment: m.Environment.ValueString(),
		Labels:      m.Labels.ValueString(),
		SendAlerts:  client.BoolPtr(m.SendAlerts.ValueBool()),
		AutoUpdate:  client.BoolPtr(m.AutoUpdate.ValueBool()),
	}
	if !m.ExtraArgs.IsNull() {
		var args []string
		m.ExtraArgs.ElementsAs(ctx, &args, false)
		cfg.ExtraArgs = args
	}
	return cfg, nil
}

func mapDeploymentToModel(ctx context.Context, d *client.Deployment, m *DeploymentResourceModel) {
	m.ID = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Name)
	m.ServerID = types.StringValue(d.Config.ServerID)
	m.ImageType = types.StringValue(d.Config.Image.Type)
	if d.Config.Image.Type == "Build" {
		m.Image = types.StringValue(d.Config.Image.Params.BuildID)
	} else {
		m.Image = types.StringValue(d.Config.Image.Params.Image)
	}
	m.Network = types.StringValue(d.Config.Network)
	m.RestartMode = types.StringValue(d.Config.RestartMode)
	m.Command = types.StringValue(d.Config.Command)

	if d.Config.SendAlerts != nil {
		m.SendAlerts = types.BoolValue(*d.Config.SendAlerts)
	}
	if d.Config.AutoUpdate != nil {
		m.AutoUpdate = types.BoolValue(*d.Config.AutoUpdate)
	}

	m.Ports = types.StringValue(d.Config.Ports)
	m.Volumes = types.StringValue(d.Config.Volumes)
	m.Environment = types.StringValue(d.Config.Environment)
	m.Labels = types.StringValue(d.Config.Labels)

	if len(d.Config.ExtraArgs) > 0 {
		list, _ := types.ListValueFrom(ctx, types.StringType, d.Config.ExtraArgs)
		m.ExtraArgs = list
	} else if !m.ExtraArgs.IsNull() {
		m.ExtraArgs = types.ListNull(types.StringType)
	}

	m.Tags = mapTagsToModel(d.Tags, m.Tags)
}

func applyDeploymentStartedState(ctx context.Context, c *client.Client, id string, m *DeploymentResourceModel, diags *diag.Diagnostics) {
	if m.Started.ValueBool() {
		if err := c.DeployDeployment(ctx, id); err != nil {
			diags.AddError("Error starting deployment", fmt.Sprintf("failed to deploy %q: %v", id, err))
		}
	} else {
		if err := c.DestroyDeployment(ctx, id); err != nil {
			diags.AddError("Error stopping deployment", fmt.Sprintf("failed to destroy %q: %v", id, err))
		}
	}
}

func applyDeploymentDeployedState(ctx context.Context, c *client.Client, id string, m *DeploymentResourceModel, diags *diag.Diagnostics) {
	if m.Deployed.ValueBool() {
		time.Sleep(5 * time.Second)
		if err := c.DeployDeployment(ctx, id); err != nil {
			diags.AddError("Error deploying deployment", fmt.Sprintf("failed to deploy %q: %v", id, err))
		}
	}
}

func (r *DeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, _ := deploymentConfigFromModel(ctx, &plan, resp)

	deployment, err := r.client.CreateDeployment(ctx, client.CreateDeploymentParams{
		Name: plan.Name.ValueString(), Config: cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating deployment", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	mapDeploymentToModel(ctx, deployment, &plan)
	applyResourceTags(ctx, r.client, "Deployment", deployment.ID, tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}

	applyDeploymentDeployedState(ctx, r.client, deployment.ID, &plan, &resp.Diagnostics)
	if !plan.Deployed.ValueBool() {
		applyDeploymentStartedState(ctx, r.client, deployment.ID, &plan, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deployment, err := r.client.GetDeployment(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading deployment", err.Error())
		return
	}

	mapDeploymentToModel(ctx, deployment, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DeploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state DeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Name.ValueString() != state.Name.ValueString() {
		err := r.client.Write(ctx, "RenameDeployment", map[string]string{
			"id": state.ID.ValueString(), "name": plan.Name.ValueString(),
		}, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error renaming deployment", err.Error())
			return
		}
	}

	// Build config from plan (reuse create helper pattern).
	imgType := plan.ImageType.ValueString()
	if imgType == "" {
		imgType = "Image"
	}
	var imgParams client.DeploymentImageParams
	if imgType == "Build" {
		imgParams.BuildID = plan.Image.ValueString()
	} else {
		imgParams.Image = plan.Image.ValueString()
	}
	cfg := client.DeploymentConfig{
		ServerID:    plan.ServerID.ValueString(),
		Image:       client.DeploymentImage{Type: imgType, Params: imgParams},
		Network:     plan.Network.ValueString(),
		RestartMode: plan.RestartMode.ValueString(),
		Command:     plan.Command.ValueString(),
		Ports:       plan.Ports.ValueString(),
		Volumes:     plan.Volumes.ValueString(),
		Environment: plan.Environment.ValueString(),
		Labels:      plan.Labels.ValueString(),
		SendAlerts:  client.BoolPtr(plan.SendAlerts.ValueBool()),
		AutoUpdate:  client.BoolPtr(plan.AutoUpdate.ValueBool()),
	}
	if !plan.ExtraArgs.IsNull() {
		var args []string
		plan.ExtraArgs.ElementsAs(ctx, &args, false)
		cfg.ExtraArgs = args
	}

	deployment, err := r.client.UpdateDeployment(ctx, client.UpdateDeploymentParams{
		ID: state.ID.ValueString(), Config: cfg,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating deployment", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	mapDeploymentToModel(ctx, deployment, &plan)
	applyResourceTags(ctx, r.client, "Deployment", state.ID.ValueString(), tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}

	applyDeploymentDeployedState(ctx, r.client, state.ID.ValueString(), &plan, &resp.Diagnostics)
	if !plan.Deployed.ValueBool() {
		applyDeploymentStartedState(ctx, r.client, state.ID.ValueString(), &plan, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DeploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteDeployment(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError("Error deleting deployment", err.Error())
	}
}

func (r *DeploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *DeploymentResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || r.client == nil {
		return
	}
	var plan DeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	changed := false

	// Resolve server name → canonical ID.
	// - If the resolved ID matches the prior state: replace plan with ID (no drift).
	// - Otherwise (new resource or server change): mark Unknown so Terraform doesn't
	//   constrain the post-apply value. Create/Update resolve before the API call.
	if !plan.ServerID.IsUnknown() && !plan.ServerID.IsNull() {
		configServerID := plan.ServerID.ValueString()
		var priorServerID string
		if !req.State.Raw.IsNull() {
			var priorState DeploymentResourceModel
			if diags := req.State.Get(ctx, &priorState); !diags.HasError() {
				priorServerID = priorState.ServerID.ValueString()
			}
		}
		if resolved := resolveServerID(ctx, r.client, configServerID); resolved != configServerID {
			if resolved == priorServerID && priorServerID != "" {
				plan.ServerID = types.StringValue(resolved)
			} else {
				plan.ServerID = types.StringUnknown()
			}
			changed = true
		}
	}

	// Normalize empty extra_args list to null so it round-trips consistently with Read.
	if !plan.ExtraArgs.IsNull() && !plan.ExtraArgs.IsUnknown() && len(plan.ExtraArgs.Elements()) == 0 {
		plan.ExtraArgs = types.ListNull(types.StringType)
		changed = true
	}

	if changed {
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
	}
}

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// useStateForUnknownIncludingNull is like UseStateForUnknown but also uses
// the state value when it is null. The standard modifier skips null state,
// causing perpetual unknown diffs for optional computed lists the API returns
// as empty/null.
type useStateForUnknownIncludingNull struct{}

func (m useStateForUnknownIncludingNull) Description(_ context.Context) string {
	return "Uses the prior state value (including null) when the planned value is unknown."
}
func (m useStateForUnknownIncludingNull) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (m useStateForUnknownIncludingNull) PlanModifyList(_ context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	// Only act when the planned value is unknown and the resource already exists.
	if !req.PlanValue.IsUnknown() || req.State.Raw.IsNull() {
		return
	}
	resp.PlanValue = req.StateValue
}

var (
	_ resource.Resource                = &StackResource{}
	_ resource.ResourceWithImportState = &StackResource{}
	_ resource.ResourceWithModifyPlan  = &StackResource{}
)

type StackResource struct {
	client *client.Client
}

type StackResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	ServerID            types.String `tfsdk:"server_id"`
	ProjectName         types.String `tfsdk:"project_name"`
	AutoPull            types.Bool   `tfsdk:"auto_pull"`
	RunBuild            types.Bool   `tfsdk:"run_build"`
	AutoUpdate          types.Bool   `tfsdk:"auto_update"`
	DestroyBeforeDeploy types.Bool   `tfsdk:"destroy_before_deploy"`
	GitProvider         types.String `tfsdk:"git_provider"`
	GitHTTPS            types.Bool   `tfsdk:"git_https"`
	GitAccount          types.String `tfsdk:"git_account"`
	Repo                types.String `tfsdk:"repo"`
	Branch              types.String `tfsdk:"branch"`
	Commit              types.String `tfsdk:"commit"`
	FilePaths           types.List   `tfsdk:"file_paths"`
	FilesOnHost         types.Bool   `tfsdk:"files_on_host"`
	FileContents        types.String `tfsdk:"file_contents"`
	Environment         types.String `tfsdk:"environment"`
	RunDirectory        types.String `tfsdk:"run_directory"`
	EnvFilePath         types.String `tfsdk:"env_file_path"`
	AdditionalEnvFiles  types.List   `tfsdk:"additional_env_files"`
	WebhookEnabled      types.Bool   `tfsdk:"webhook_enabled"`
	WebhookSecret       types.String `tfsdk:"webhook_secret"`
	WebhookForceDeploy  types.Bool   `tfsdk:"webhook_force_deploy"`
	PreDeployPath       types.String `tfsdk:"pre_deploy_path"`
	PreDeployCommand    types.String `tfsdk:"pre_deploy_command"`
	PostDeployPath      types.String `tfsdk:"post_deploy_path"`
	PostDeployCommand   types.String `tfsdk:"post_deploy_command"`
	ExtraArgs           types.List   `tfsdk:"extra_args"`
	BuildExtraArgs      types.List   `tfsdk:"build_extra_args"`
	IgnoreServices      types.List   `tfsdk:"ignore_services"`
	SendAlerts          types.Bool   `tfsdk:"send_alerts"`
	Started             types.Bool   `tfsdk:"started"`
	Deployed            types.Bool   `tfsdk:"deployed"`
	Tags                types.List   `tfsdk:"tags"`
}

func NewStackResource() resource.Resource {
	return &StackResource{}
}

func (r *StackResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack"
}

func (r *StackResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Komodo stack (Docker Compose stack).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the stack.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Display name of the stack.",
				Required:    true,
			},
			"server_id": schema.StringAttribute{
				Description: "The server to deploy the stack on (ID or name).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Local"),
			},
			"project_name": schema.StringAttribute{
				Description: "Custom project name for docker compose -p. Defaults to the stack name.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"auto_pull": schema.BoolAttribute{
				Description: "Whether to automatically compose pull before redeploying.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"run_build": schema.BoolAttribute{
				Description: "Whether to docker compose build before deploy.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"auto_update": schema.BoolAttribute{
				Description: "Whether to automatically redeploy when newer images are found.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"destroy_before_deploy": schema.BoolAttribute{
				Description: "Whether to run docker compose down before compose up.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"git_provider": schema.StringAttribute{
				Description: "Git provider domain (e.g., github.com).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("github.com"),
			},
			"git_https": schema.BoolAttribute{
				Description: "Whether to use HTTPS to clone the repo.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"git_account": schema.StringAttribute{
				Description: "Git account for accessing private repos.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"repo": schema.StringAttribute{
				Description: "Git repository in namespace/repo_name format.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"branch": schema.StringAttribute{
				Description: "Branch of the repo.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("main"),
			},
			"commit": schema.StringAttribute{
				Description: "Specific commit hash.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"file_paths": schema.ListAttribute{
				Description: "Paths to docker-compose.yml files.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					useStateForUnknownIncludingNull{},
				},
			},
			"files_on_host": schema.BoolAttribute{
				Description: "Source files from the host instead of a git repo.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"file_contents": schema.StringAttribute{
				Description: "Inline compose file contents for UI-managed stacks (alternative to repo-based files).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"environment": schema.StringAttribute{
				Description: "Environment variables written to the env file before docker compose up (KEY=VALUE format, newline separated).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"run_directory": schema.StringAttribute{
				Description: "Directory to cd into before running docker compose.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"env_file_path": schema.StringAttribute{
				Description: "Name of the environment file written before docker compose up.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(".env"),
			},
			"additional_env_files": schema.ListAttribute{
				Description: "Additional env files to attach with --env-file.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					useStateForUnknownIncludingNull{},
				},
			},
			"webhook_enabled": schema.BoolAttribute{
				Description: "Whether incoming webhooks trigger action.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"webhook_secret": schema.StringAttribute{
				Description: "Alternate webhook secret for this stack.",
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
				Default:     stringdefault.StaticString(""),
			},
			"webhook_force_deploy": schema.BoolAttribute{
				Description: "Always execute deployment without comparing changes when webhook fires.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"pre_deploy_path": schema.StringAttribute{
				Description: "Working directory for the pre-deploy command.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"pre_deploy_command": schema.StringAttribute{
				Description: "Command to run before the stack is deployed.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"post_deploy_path": schema.StringAttribute{
				Description: "Working directory for the post-deploy command.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"post_deploy_command": schema.StringAttribute{
				Description: "Command to run after the stack is deployed.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"extra_args": schema.ListAttribute{
				Description: "Extra arguments passed after docker compose up -d.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					useStateForUnknownIncludingNull{},
				},
			},
			"build_extra_args": schema.ListAttribute{
				Description: "Extra arguments passed after docker compose build.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					useStateForUnknownIncludingNull{},
				},
			},
			"ignore_services": schema.ListAttribute{
				Description: "Services to exclude from stack health status checks.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					useStateForUnknownIncludingNull{},
				},
			},
			"send_alerts": schema.BoolAttribute{
				Description: "Whether to send StackStateChange alerts.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"started": schema.BoolAttribute{
				Description: "Whether the stack should be running. When true, runs DeployStack (compose up) to create and start containers. When false, runs DestroyStack (compose down) to stop and remove them. Intent-based and async.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"deployed": schema.BoolAttribute{
				Description: "When true, triggers a full DeployStack (git pull + compose up) after create/update. Use this to redeploy on config changes. Like started, this is intent-based and async.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"tags": schema.ListAttribute{
				Description: "List of tag names to apply to this stack.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *StackResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

func stackConfigFromModel(ctx context.Context, m *StackResourceModel) client.StackConfig {
	serverID := m.ServerID.ValueString()
	if serverID == "" {
		serverID = "Local"
	}

	cfg := client.StackConfig{
		ServerID:            serverID,
		ProjectName:         m.ProjectName.ValueString(),
		AutoPull:            client.BoolPtr(m.AutoPull.ValueBool()),
		RunBuild:            client.BoolPtr(m.RunBuild.ValueBool()),
		AutoUpdate:          client.BoolPtr(m.AutoUpdate.ValueBool()),
		DestroyBeforeDeploy: client.BoolPtr(m.DestroyBeforeDeploy.ValueBool()),
		GitProvider:         m.GitProvider.ValueString(),
		GitHTTPS:            client.BoolPtr(m.GitHTTPS.ValueBool()),
		GitAccount:          m.GitAccount.ValueString(),
		Repo:                m.Repo.ValueString(),
		Branch:              m.Branch.ValueString(),
		Commit:              m.Commit.ValueString(),
		FilesOnHost:         client.BoolPtr(m.FilesOnHost.ValueBool()),
		FileContents:        m.FileContents.ValueString(),
		Environment:         m.Environment.ValueString(),
		RunDirectory:        m.RunDirectory.ValueString(),
		EnvFilePath:         m.EnvFilePath.ValueString(),
		WebhookEnabled:      client.BoolPtr(m.WebhookEnabled.ValueBool()),
		WebhookSecret:       m.WebhookSecret.ValueString(),
		WebhookForceDeploy:  client.BoolPtr(m.WebhookForceDeploy.ValueBool()),
		SendAlerts:          client.BoolPtr(m.SendAlerts.ValueBool()),
	}

	if !m.FilePaths.IsNull() {
		var paths []string
		m.FilePaths.ElementsAs(ctx, &paths, false)
		cfg.FilePaths = paths
	}

	if !m.AdditionalEnvFiles.IsNull() {
		var v []string
		m.AdditionalEnvFiles.ElementsAs(ctx, &v, false)
		cfg.AdditionalEnvFiles = v
	}

	if !m.ExtraArgs.IsNull() {
		var v []string
		m.ExtraArgs.ElementsAs(ctx, &v, false)
		cfg.ExtraArgs = v
	}

	if !m.BuildExtraArgs.IsNull() {
		var v []string
		m.BuildExtraArgs.ElementsAs(ctx, &v, false)
		cfg.BuildExtraArgs = v
	}

	if !m.IgnoreServices.IsNull() {
		var v []string
		m.IgnoreServices.ElementsAs(ctx, &v, false)
		cfg.IgnoreServices = v
	}

	if m.PreDeployCommand.ValueString() != "" {
		cfg.PreDeploy = &client.SystemCommand{
			Path:    m.PreDeployPath.ValueString(),
			Command: m.PreDeployCommand.ValueString(),
		}
	}

	if m.PostDeployCommand.ValueString() != "" {
		cfg.PostDeploy = &client.SystemCommand{
			Path:    m.PostDeployPath.ValueString(),
			Command: m.PostDeployCommand.ValueString(),
		}
	}

	return cfg
}

// applyStartedState deploys or destroys the stack to match m.Started.
// DeployStack (compose up) creates and starts containers; DestroyStack (compose down) removes them.
// StartStack/StopStack are not used here because they only work on pre-existing containers.
func applyStartedState(ctx context.Context, c *client.Client, stackID string, m *StackResourceModel, diags *diag.Diagnostics) {
	if m.Started.ValueBool() {
		if err := c.DeployStack(ctx, stackID); err != nil {
			diags.AddError("Error starting stack", fmt.Sprintf("failed to deploy stack %q: %v", stackID, err))
		}
	} else {
		if err := c.DestroyStack(ctx, stackID); err != nil {
			diags.AddError("Error stopping stack", fmt.Sprintf("failed to destroy stack %q: %v", stackID, err))
		}
	}
}

// applyDeployedState triggers a full DeployStack (git pull + compose up) when m.Deployed is true.
// A short delay before the call lets Komodo finish registering the stack before accepting execute actions.
func applyDeployedState(ctx context.Context, c *client.Client, stackID string, m *StackResourceModel, diags *diag.Diagnostics) {
	if m.Deployed.ValueBool() {
		time.Sleep(5 * time.Second)
		if err := c.DeployStack(ctx, stackID); err != nil {
			diags.AddError("Error deploying stack", fmt.Sprintf("failed to deploy stack %q: %v", stackID, err))
		}
	}
}

func mapListToModel(vals []string, current types.List) types.List {
	if len(vals) > 0 {
		list, _ := types.ListValueFrom(context.Background(), types.StringType, vals)
		return list
	}
	if !current.IsNull() {
		return types.ListNull(types.StringType)
	}
	return current
}

func mapStackToModel(s *client.Stack, m *StackResourceModel) {
	m.ID = types.StringValue(s.ID)
	m.Name = types.StringValue(s.Name)
	m.ServerID = types.StringValue(s.Config.ServerID)
	m.ProjectName = types.StringValue(s.Config.ProjectName)
	m.GitProvider = types.StringValue(s.Config.GitProvider)
	m.GitAccount = types.StringValue(s.Config.GitAccount)
	m.Repo = types.StringValue(s.Config.Repo)
	m.Branch = types.StringValue(s.Config.Branch)
	m.Commit = types.StringValue(s.Config.Commit)
	// file_contents is not overridden from the API — the .tf config is authoritative.
	// The API may normalize whitespace/newlines, which would cause perpetual diffs.
	m.Environment = types.StringValue(strings.TrimRight(s.Config.Environment, "\n"))
	m.RunDirectory = types.StringValue(s.Config.RunDirectory)
	m.EnvFilePath = types.StringValue(s.Config.EnvFilePath)
	m.WebhookSecret = types.StringValue(s.Config.WebhookSecret)

	m.FilePaths = mapListToModel(s.Config.FilePaths, m.FilePaths)
	m.AdditionalEnvFiles = mapListToModel(s.Config.AdditionalEnvFiles, m.AdditionalEnvFiles)
	m.ExtraArgs = mapListToModel(s.Config.ExtraArgs, m.ExtraArgs)
	m.BuildExtraArgs = mapListToModel(s.Config.BuildExtraArgs, m.BuildExtraArgs)
	m.IgnoreServices = mapListToModel(s.Config.IgnoreServices, m.IgnoreServices)

	if s.Config.AutoPull != nil {
		m.AutoPull = types.BoolValue(*s.Config.AutoPull)
	}
	if s.Config.RunBuild != nil {
		m.RunBuild = types.BoolValue(*s.Config.RunBuild)
	}
	if s.Config.AutoUpdate != nil {
		m.AutoUpdate = types.BoolValue(*s.Config.AutoUpdate)
	}
	if s.Config.DestroyBeforeDeploy != nil {
		m.DestroyBeforeDeploy = types.BoolValue(*s.Config.DestroyBeforeDeploy)
	}
	if s.Config.GitHTTPS != nil {
		m.GitHTTPS = types.BoolValue(*s.Config.GitHTTPS)
	}
	if s.Config.FilesOnHost != nil {
		m.FilesOnHost = types.BoolValue(*s.Config.FilesOnHost)
	}
	if s.Config.WebhookEnabled != nil {
		m.WebhookEnabled = types.BoolValue(*s.Config.WebhookEnabled)
	}
	if s.Config.WebhookForceDeploy != nil {
		m.WebhookForceDeploy = types.BoolValue(*s.Config.WebhookForceDeploy)
	}
	if s.Config.SendAlerts != nil {
		m.SendAlerts = types.BoolValue(*s.Config.SendAlerts)
	}

	if s.Config.PreDeploy != nil {
		m.PreDeployPath = types.StringValue(strings.TrimRight(s.Config.PreDeploy.Path, "\n"))
		m.PreDeployCommand = types.StringValue(strings.TrimRight(s.Config.PreDeploy.Command, "\n"))
	}
	if s.Config.PostDeploy != nil {
		m.PostDeployPath = types.StringValue(strings.TrimRight(s.Config.PostDeploy.Path, "\n"))
		m.PostDeployCommand = types.StringValue(strings.TrimRight(s.Config.PostDeploy.Command, "\n"))
	}

	m.Tags = mapTagsToModel(s.Tags, m.Tags)
}

func (r *StackResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan StackResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if resolved := resolveServerID(ctx, r.client, plan.ServerID.ValueString()); resolved != plan.ServerID.ValueString() {
		plan.ServerID = types.StringValue(resolved)
	}

	params := client.CreateStackParams{
		Name:   plan.Name.ValueString(),
		Config: stackConfigFromModel(ctx, &plan),
	}

	stack, err := r.client.CreateStack(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating stack",
			fmt.Sprintf("failed creating stack %q on server %q: %v", plan.Name.ValueString(), plan.ServerID.ValueString(), err),
		)
		return
	}

	tagsFromPlan := plan.Tags
	mapStackToModel(stack, &plan)
	applyResourceTags(ctx, r.client, "Stack", stack.ID, tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}

	// Deploy first (git pull + compose up), then ensure started state.
	// If deployed already triggered a DeployStack, skip applyStartedState to
	// avoid a "Resource is busy" error from double-calling the API.
	applyDeployedState(ctx, r.client, stack.ID, &plan, &resp.Diagnostics)
	if !plan.Deployed.ValueBool() {
		applyStartedState(ctx, r.client, stack.ID, &plan, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StackResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state StackResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.ID.IsNull() || state.ID.IsUnknown() || state.ID.ValueString() == "" {
		// If we don't have an ID yet, the resource isn't created yet.
		resp.State.RemoveResource(ctx)
		return
	}

	if state.ServerID.ValueString() == "" {
		state.ServerID = types.StringValue("Local")
	}

	stack, err := r.client.GetStack(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading stack",
			fmt.Sprintf("failed reading stack %q (id=%q) from server %q: %v", state.Name.ValueString(), state.ID.ValueString(), state.ServerID.ValueString(), err),
		)
		return
	}

	mapStackToModel(stack, &state)
	// `started` is an intent attribute — leave it as whatever is in state.
	// StartStack/StopStack are async so reading container state immediately
	// after would race. Drift is surfaced the next time apply is run.
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *StackResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan StackResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state StackResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Name.ValueString() != state.Name.ValueString() {
		err := r.client.Write(ctx, "RenameStack", map[string]string{
			"id": state.ID.ValueString(), "name": plan.Name.ValueString(),
		}, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error renaming stack", err.Error())
			return
		}
	}

	if resolved := resolveServerID(ctx, r.client, plan.ServerID.ValueString()); resolved != plan.ServerID.ValueString() {
		plan.ServerID = types.StringValue(resolved)
	}

	params := client.UpdateStackParams{
		ID:     state.ID.ValueString(),
		Config: stackConfigFromModel(ctx, &plan),
	}

	stack, err := r.client.UpdateStack(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating stack",
			fmt.Sprintf("failed updating stack %q (id=%q) on server %q: %v", plan.Name.ValueString(), state.ID.ValueString(), plan.ServerID.ValueString(), err),
		)
		return
	}

	tagsFromPlan := plan.Tags
	mapStackToModel(stack, &plan)
	applyResourceTags(ctx, r.client, "Stack", state.ID.ValueString(), tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}

	// Deploy first (git pull + compose up), then ensure started state.
	// If deployed already triggered a DeployStack, skip applyStartedState to
	// avoid a "Resource is busy" error from double-calling the API.
	applyDeployedState(ctx, r.client, state.ID.ValueString(), &plan, &resp.Diagnostics)
	if !plan.Deployed.ValueBool() {
		applyStartedState(ctx, r.client, state.ID.ValueString(), &plan, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *StackResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state StackResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteStack(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting stack",
			fmt.Sprintf("failed deleting stack %q (id=%q) from server %q: %v", state.Name.ValueString(), state.ID.ValueString(), state.ServerID.ValueString(), err),
		)
	}
}

func (r *StackResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *StackResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip on destroy or when client is not yet configured.
	if req.Plan.Raw.IsNull() || r.client == nil {
		return
	}
	var plan StackResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	changed := false

	// Suppress server name→ID drift for existing resources: if the configured
	// value (e.g. "dockerai") resolves to the canonical ID already in state,
	// use the state value so Terraform sees no change.
	// For new resources the config value is left as-is; Create/Update resolve
	// the name before the API call and then restore the original value into
	// state so that "dockerai" round-trips without needing a data source.
	if !plan.ServerID.IsUnknown() && !plan.ServerID.IsNull() {
		configServerID := plan.ServerID.ValueString()
		var priorServerID string
		if !req.State.Raw.IsNull() {
			var priorState StackResourceModel
			if diags := req.State.Get(ctx, &priorState); !diags.HasError() {
				priorServerID = priorState.ServerID.ValueString()
			}
		}
		if configServerID != priorServerID && priorServerID != "" {
			if resolved := resolveServerID(ctx, r.client, configServerID); resolved == priorServerID {
				plan.ServerID = types.StringValue(priorServerID)
				changed = true
			}
		}
	}

	// Normalize empty lists to null so they round-trip consistently
	// (the API treats [] and unset identically).
	for _, lp := range []*types.List{
		&plan.FilePaths, &plan.AdditionalEnvFiles,
		&plan.ExtraArgs, &plan.BuildExtraArgs, &plan.IgnoreServices,
	} {
		if !lp.IsNull() && !lp.IsUnknown() && len(lp.Elements()) == 0 {
			*lp = types.ListNull(types.StringType)
			changed = true
		}
	}

	if changed {
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
	}
}

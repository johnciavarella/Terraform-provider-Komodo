package provider

import (
	"context"
	"errors"
	"fmt"

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
	_ resource.Resource                = &BuildResource{}
	_ resource.ResourceWithImportState = &BuildResource{}
)

type BuildResource struct {
	client *client.Client
}

type BuildResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	BuilderID            types.String `tfsdk:"builder_id"`
	ImageName            types.String `tfsdk:"image_name"`
	ImageTag             types.String `tfsdk:"image_tag"`
	AutoIncrementVersion types.Bool   `tfsdk:"auto_increment_version"`
	GitProvider          types.String `tfsdk:"git_provider"`
	GitHTTPS             types.Bool   `tfsdk:"git_https"`
	GitAccount           types.String `tfsdk:"git_account"`
	Repo                 types.String `tfsdk:"repo"`
	Branch               types.String `tfsdk:"branch"`
	Commit               types.String `tfsdk:"commit"`
	BuildPath            types.String `tfsdk:"build_path"`
	DockerfilePath       types.String `tfsdk:"dockerfile_path"`
	UseBuildx            types.Bool   `tfsdk:"use_buildx"`
	BuildArgs            types.String `tfsdk:"build_args"`
	Labels               types.String `tfsdk:"labels"`
	WebhookEnabled       types.Bool   `tfsdk:"webhook_enabled"`
	FilesOnHost          types.Bool   `tfsdk:"files_on_host"`
	Tags                 types.List   `tfsdk:"tags"`
}

func NewBuildResource() resource.Resource {
	return &BuildResource{}
}

func (r *BuildResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_build"
}

func (r *BuildResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Komodo build (Docker image build configuration).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Unique identifier.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":                   schema.StringAttribute{Required: true, Description: "Display name."},
			"builder_id":             schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Builder used to build the image (ID or name)."},
			"image_name":             schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Alternate image name for the registry."},
			"image_tag":              schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Extra tag after the build version."},
			"auto_increment_version": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Auto-increment patch version on every build."},
			"git_provider":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("github.com"), Description: "Git provider domain."},
			"git_https":              schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Use HTTPS to clone."},
			"git_account":            schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Git account for private repos."},
			"repo":                   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Git repository (namespace/repo_name)."},
			"branch":                 schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Branch of the repo."},
			"commit":                 schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Specific commit hash."},
			"build_path":             schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Build context path."},
			"dockerfile_path":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Path to Dockerfile."},
			"use_buildx":             schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Use docker buildx."},
			"build_args":             schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Build arguments."},
			"labels":                 schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Image labels."},
			"webhook_enabled":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Enable webhooks."},
			"files_on_host":          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Source files from host."},
			"tags":                   schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "List of tag names to apply to this build."},
		},
	}
}

func (r *BuildResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func buildConfigFromModel(m *BuildResourceModel) client.BuildConfig {
	return client.BuildConfig{
		BuilderID:            m.BuilderID.ValueString(),
		ImageName:            m.ImageName.ValueString(),
		ImageTag:             m.ImageTag.ValueString(),
		AutoIncrementVersion: client.BoolPtr(m.AutoIncrementVersion.ValueBool()),
		GitProvider:          m.GitProvider.ValueString(),
		GitHTTPS:             client.BoolPtr(m.GitHTTPS.ValueBool()),
		GitAccount:           m.GitAccount.ValueString(),
		Repo:                 m.Repo.ValueString(),
		Branch:               m.Branch.ValueString(),
		Commit:               m.Commit.ValueString(),
		BuildPath:            m.BuildPath.ValueString(),
		DockerfilePath:       m.DockerfilePath.ValueString(),
		UseBuildx:            client.BoolPtr(m.UseBuildx.ValueBool()),
		BuildArgs:            m.BuildArgs.ValueString(),
		Labels:               m.Labels.ValueString(),
		WebhookEnabled:       client.BoolPtr(m.WebhookEnabled.ValueBool()),
		FilesOnHost:          client.BoolPtr(m.FilesOnHost.ValueBool()),
	}
}

func mapBuildToModel(b *client.Build, m *BuildResourceModel) {
	m.ID = types.StringValue(b.ID)
	m.Name = types.StringValue(b.Name)
	m.BuilderID = types.StringValue(b.Config.BuilderID)
	m.ImageName = types.StringValue(b.Config.ImageName)
	m.ImageTag = types.StringValue(b.Config.ImageTag)
	m.GitProvider = types.StringValue(b.Config.GitProvider)
	m.GitAccount = types.StringValue(b.Config.GitAccount)
	m.Repo = types.StringValue(b.Config.Repo)
	m.Branch = types.StringValue(b.Config.Branch)
	m.Commit = types.StringValue(b.Config.Commit)
	m.BuildPath = types.StringValue(b.Config.BuildPath)
	m.DockerfilePath = types.StringValue(b.Config.DockerfilePath)
	m.BuildArgs = types.StringValue(b.Config.BuildArgs)
	m.Labels = types.StringValue(b.Config.Labels)

	if b.Config.AutoIncrementVersion != nil {
		m.AutoIncrementVersion = types.BoolValue(*b.Config.AutoIncrementVersion)
	}
	if b.Config.GitHTTPS != nil {
		m.GitHTTPS = types.BoolValue(*b.Config.GitHTTPS)
	}
	if b.Config.UseBuildx != nil {
		m.UseBuildx = types.BoolValue(*b.Config.UseBuildx)
	}
	if b.Config.WebhookEnabled != nil {
		m.WebhookEnabled = types.BoolValue(*b.Config.WebhookEnabled)
	}
	if b.Config.FilesOnHost != nil {
		m.FilesOnHost = types.BoolValue(*b.Config.FilesOnHost)
	}

	m.Tags = mapTagsToModel(b.Tags, m.Tags)
}

func (r *BuildResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BuildResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	build, err := r.client.CreateBuild(ctx, client.CreateBuildParams{
		Name: plan.Name.ValueString(), Config: buildConfigFromModel(&plan),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating build", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	mapBuildToModel(build, &plan)
	applyResourceTags(ctx, r.client, "Build", build.ID, tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BuildResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BuildResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	build, err := r.client.GetBuild(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading build", err.Error())
		return
	}

	mapBuildToModel(build, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BuildResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BuildResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state BuildResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Name.ValueString() != state.Name.ValueString() {
		err := r.client.Write(ctx, "RenameBuild", map[string]string{
			"id": state.ID.ValueString(), "name": plan.Name.ValueString(),
		}, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error renaming build", err.Error())
			return
		}
	}

	build, err := r.client.UpdateBuild(ctx, client.UpdateBuildParams{
		ID: state.ID.ValueString(), Config: buildConfigFromModel(&plan),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating build", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	mapBuildToModel(build, &plan)
	applyResourceTags(ctx, r.client, "Build", state.ID.ValueString(), tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BuildResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BuildResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteBuild(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError("Error deleting build", err.Error())
	}
}

func (r *BuildResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

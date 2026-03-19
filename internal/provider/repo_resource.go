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
	_ resource.Resource                = &RepoResource{}
	_ resource.ResourceWithImportState = &RepoResource{}
	_ resource.ResourceWithModifyPlan  = &RepoResource{}
)

type RepoResource struct {
	client *client.Client
}

type RepoResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	ServerID       types.String `tfsdk:"server_id"`
	GitProvider    types.String `tfsdk:"git_provider"`
	GitHTTPS       types.Bool   `tfsdk:"git_https"`
	GitAccount     types.String `tfsdk:"git_account"`
	Repo           types.String `tfsdk:"repo"`
	Branch         types.String `tfsdk:"branch"`
	Commit         types.String `tfsdk:"commit"`
	OnClone        types.String `tfsdk:"on_clone"`
	OnPull         types.String `tfsdk:"on_pull"`
	WebhookEnabled types.Bool   `tfsdk:"webhook_enabled"`
	Tags           types.List   `tfsdk:"tags"`
}

func NewRepoResource() resource.Resource {
	return &RepoResource{}
}

func (r *RepoResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_repo"
}

func (r *RepoResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Komodo repo (a git repository cloned on a server).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Unique identifier.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":            schema.StringAttribute{Required: true, Description: "Display name."},
			"server_id":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Server to clone onto (ID or name)."},
			"git_provider":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("github.com"), Description: "Git provider domain."},
			"git_https":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Use HTTPS to clone."},
			"git_account":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Git account for private repos."},
			"repo":            schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Git repository (namespace/repo_name)."},
			"branch":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Branch."},
			"commit":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Specific commit hash."},
			"on_clone":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Command to run after cloning."},
			"on_pull":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Command to run after pulling."},
			"webhook_enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Enable webhooks."},
			"tags":            schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "List of tag names to apply to this repo."},
		},
	}
}

func (r *RepoResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func repoConfigFromModel(m *RepoResourceModel) client.RepoConfig {
	return client.RepoConfig{
		ServerID:       m.ServerID.ValueString(),
		GitProvider:    m.GitProvider.ValueString(),
		GitHTTPS:       client.BoolPtr(m.GitHTTPS.ValueBool()),
		GitAccount:     m.GitAccount.ValueString(),
		Repo:           m.Repo.ValueString(),
		Branch:         m.Branch.ValueString(),
		Commit:         m.Commit.ValueString(),
		OnClone:        m.OnClone.ValueString(),
		OnPull:         m.OnPull.ValueString(),
		WebhookEnabled: client.BoolPtr(m.WebhookEnabled.ValueBool()),
	}
}

func mapRepoToModel(r *client.Repo, m *RepoResourceModel) {
	m.ID = types.StringValue(r.ID)
	m.Name = types.StringValue(r.Name)
	m.ServerID = types.StringValue(r.Config.ServerID)
	m.GitProvider = types.StringValue(r.Config.GitProvider)
	m.GitAccount = types.StringValue(r.Config.GitAccount)
	m.Repo = types.StringValue(r.Config.Repo)
	m.Branch = types.StringValue(r.Config.Branch)
	m.Commit = types.StringValue(r.Config.Commit)
	m.OnClone = types.StringValue(r.Config.OnClone)
	m.OnPull = types.StringValue(r.Config.OnPull)

	if r.Config.GitHTTPS != nil {
		m.GitHTTPS = types.BoolValue(*r.Config.GitHTTPS)
	}
	if r.Config.WebhookEnabled != nil {
		m.WebhookEnabled = types.BoolValue(*r.Config.WebhookEnabled)
	}

	m.Tags = mapTagsToModel(r.Tags, m.Tags)
}

func (r *RepoResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RepoResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repo, err := r.client.CreateRepo(ctx, client.CreateRepoParams{
		Name: plan.Name.ValueString(), Config: repoConfigFromModel(&plan),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating repo", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	mapRepoToModel(repo, &plan)
	applyResourceTags(ctx, r.client, "Repo", repo.ID, tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RepoResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RepoResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repo, err := r.client.GetRepo(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading repo", err.Error())
		return
	}

	mapRepoToModel(repo, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RepoResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RepoResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state RepoResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Name.ValueString() != state.Name.ValueString() {
		err := r.client.Write(ctx, "RenameRepo", map[string]string{
			"id": state.ID.ValueString(), "name": plan.Name.ValueString(),
		}, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error renaming repo", err.Error())
			return
		}
	}

	repo, err := r.client.UpdateRepo(ctx, client.UpdateRepoParams{
		ID: state.ID.ValueString(), Config: repoConfigFromModel(&plan),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating repo", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	mapRepoToModel(repo, &plan)
	applyResourceTags(ctx, r.client, "Repo", state.ID.ValueString(), tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RepoResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RepoResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteRepo(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError("Error deleting repo", err.Error())
	}
}

func (r *RepoResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *RepoResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || r.client == nil {
		return
	}
	var plan RepoResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.ServerID.IsUnknown() || plan.ServerID.IsNull() {
		return
	}
	configServerID := plan.ServerID.ValueString()
	var priorServerID string
	if !req.State.Raw.IsNull() {
		var priorState RepoResourceModel
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
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
	}
}

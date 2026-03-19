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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moghtech/terraform-provider-komodo/internal/client"
)

var (
	_ resource.Resource                = &GitProviderAccountResource{}
	_ resource.ResourceWithImportState = &GitProviderAccountResource{}
)

type GitProviderAccountResource struct {
	client *client.Client
}

type GitProviderAccountResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Domain   types.String `tfsdk:"domain"`
	Username types.String `tfsdk:"username"`
	Token    types.String `tfsdk:"token"`
	Https    types.Bool   `tfsdk:"https"`
}

func NewGitProviderAccountResource() resource.Resource {
	return &GitProviderAccountResource{}
}

func (r *GitProviderAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_git_provider_account"
}

func (r *GitProviderAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Komodo git provider account (credentials for accessing git repositories).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the git provider account.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				Description: "Git provider domain without protocol (e.g. 'github.com', 'gitea.example.com'). Changing this forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"username": schema.StringAttribute{
				Description: "Git account username. Changing this forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"token": schema.StringAttribute{
				Description: "Personal access token or password for the git account. Sensitive.",
				Required:    true,
				Sensitive:   true,
			},
			"https": schema.BoolAttribute{
				Description: "Whether to access the git provider over HTTPS (true) or HTTP (false).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
		},
	}
}

func (r *GitProviderAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func mapGitProviderAccountToModel(g *client.GitProviderAccount, m *GitProviderAccountResourceModel) {
	m.ID = types.StringValue(g.ID)
	m.Domain = types.StringValue(g.Domain)
	m.Username = types.StringValue(g.Username)
	m.Https = types.BoolValue(g.Https)
	// Token is write-only — do not override from API (it is not returned)
}

func (r *GitProviderAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GitProviderAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := r.client.CreateGitProviderAccount(ctx,
		plan.Domain.ValueString(),
		plan.Username.ValueString(),
		plan.Token.ValueString(),
		plan.Https.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error creating git provider account", err.Error())
		return
	}

	mapGitProviderAccountToModel(g, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GitProviderAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GitProviderAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := r.client.GetGitProviderAccount(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading git provider account", err.Error())
		return
	}

	mapGitProviderAccountToModel(g, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *GitProviderAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GitProviderAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state GitProviderAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := r.client.UpdateGitProviderAccount(ctx,
		state.ID.ValueString(),
		plan.Domain.ValueString(),
		plan.Username.ValueString(),
		plan.Token.ValueString(),
		plan.Https.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error updating git provider account", err.Error())
		return
	}

	mapGitProviderAccountToModel(g, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GitProviderAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *GitProviderAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GitProviderAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteGitProviderAccount(ctx, state.ID.ValueString()); err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError("Error deleting git provider account",
			fmt.Sprintf("failed deleting git provider account %q/%q: %v", state.Domain.ValueString(), state.Username.ValueString(), err))
	}
}

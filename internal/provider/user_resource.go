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
	_ resource.Resource                = &UserResource{}
	_ resource.ResourceWithImportState = &UserResource{}
)

type UserResource struct {
	client *client.Client
}

type UserResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	Username                types.String `tfsdk:"username"`
	UserType                types.String `tfsdk:"user_type"`
	Description             types.String `tfsdk:"description"`
	Password                types.String `tfsdk:"password"`
	Enabled                 types.Bool   `tfsdk:"enabled"`
	Admin                   types.Bool   `tfsdk:"admin"`
	SuperAdmin              types.Bool   `tfsdk:"super_admin"`
	CreateServerPermissions types.Bool   `tfsdk:"create_server_permissions"`
	CreateBuildPermissions  types.Bool   `tfsdk:"create_build_permissions"`
}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Komodo user. Supports local (password) and service (API key) user types.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the user.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				Description: "Globally unique username. Changing this forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_type": schema.StringAttribute{
				Description: `User type: "local" (password-based) or "service" (API key-based). Changing this forces a new resource.`,
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("service"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Description for service users.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"password": schema.StringAttribute{
				Description: "Password for local users. Sensitive. Not applicable to service users.",
				Optional:    true,
				Sensitive:   true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the user is enabled and able to access the API.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"admin": schema.BoolAttribute{
				Description: "Whether the user has global admin permissions. Requires super-admin credentials to set.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"super_admin": schema.BoolAttribute{
				Description: "Whether the user is a super-admin. Read-only — managed outside Terraform.",
				Computed:    true,
			},
			"create_server_permissions": schema.BoolAttribute{
				Description: "Whether the user has permission to create servers.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"create_build_permissions": schema.BoolAttribute{
				Description: "Whether the user has permission to create builds.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func mapUserToModel(u *client.User, m *UserResourceModel) {
	m.ID = types.StringValue(u.ID)
	m.Username = types.StringValue(u.Username)
	m.Enabled = types.BoolValue(u.Enabled)
	m.Admin = types.BoolValue(u.Admin)
	m.SuperAdmin = types.BoolValue(u.SuperAdmin)
	m.CreateServerPermissions = types.BoolValue(u.CreateServerPermissions)
	m.CreateBuildPermissions = types.BoolValue(u.CreateBuildPermissions)
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var u *client.User
	var err error

	switch plan.UserType.ValueString() {
	case "local":
		u, err = r.client.CreateLocalUser(ctx, plan.Username.ValueString(), plan.Password.ValueString())
	default: // "service"
		u, err = r.client.CreateServiceUser(ctx, plan.Username.ValueString(), plan.Description.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Error creating user", err.Error())
		return
	}

	// Apply base permissions if non-default.
	if err := r.client.UpdateUserBasePermissions(ctx, u.ID,
		plan.Enabled.ValueBool(),
		plan.CreateServerPermissions.ValueBool(),
		plan.CreateBuildPermissions.ValueBool(),
	); err != nil {
		resp.Diagnostics.AddError("Error setting user permissions", err.Error())
		return
	}

	// Apply admin flag if requested.
	if plan.Admin.ValueBool() {
		if err := r.client.UpdateUserAdmin(ctx, u.ID, true); err != nil {
			resp.Diagnostics.AddError("Error setting user admin", err.Error())
			return
		}
	}

	// Re-read to get final state.
	u, err = r.client.FindUser(ctx, u.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading user after create", err.Error())
		return
	}

	mapUserToModel(u, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	u, err := r.client.FindUser(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading user", err.Error())
		return
	}

	mapUserToModel(u, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateUserBasePermissions(ctx, state.ID.ValueString(),
		plan.Enabled.ValueBool(),
		plan.CreateServerPermissions.ValueBool(),
		plan.CreateBuildPermissions.ValueBool(),
	); err != nil {
		resp.Diagnostics.AddError("Error updating user permissions", err.Error())
		return
	}

	if plan.Admin.ValueBool() != state.Admin.ValueBool() {
		if err := r.client.UpdateUserAdmin(ctx, state.ID.ValueString(), plan.Admin.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Error updating user admin", err.Error())
			return
		}
	}

	u, err := r.client.FindUser(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading user after update", err.Error())
		return
	}

	mapUserToModel(u, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteUser(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError("Error deleting user", fmt.Sprintf("failed deleting user %q: %v", state.Username.ValueString(), err))
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

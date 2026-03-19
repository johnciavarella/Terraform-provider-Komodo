package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moghtech/terraform-provider-komodo/internal/client"
)

var (
	_ resource.Resource                = &BuilderResource{}
	_ resource.ResourceWithImportState = &BuilderResource{}
	_ resource.ResourceWithModifyPlan  = &BuilderResource{}
)

type BuilderResource struct {
	client *client.Client
}

type BuilderResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	ServerID types.String `tfsdk:"server_id"`
	Tags     types.List   `tfsdk:"tags"`
}

func NewBuilderResource() resource.Resource {
	return &BuilderResource{}
}

func (r *BuilderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_builder"
}

func (r *BuilderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Komodo builder (a server used to build Docker images).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Unique identifier.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":      schema.StringAttribute{Required: true, Description: "Display name."},
			"server_id": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Server used for building (ID or name)."},
			"tags":      schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "List of tag names to apply to this builder."},
		},
	}
}

func (r *BuilderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BuilderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BuilderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	builder, err := r.client.CreateBuilder(ctx, client.CreateBuilderParams{
		Name:   plan.Name.ValueString(),
		Config: client.BuilderConfig{ServerID: plan.ServerID.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating builder", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	plan.ID = types.StringValue(builder.ID)
	plan.Name = types.StringValue(builder.Name)
	plan.ServerID = types.StringValue(builder.Config.ServerID)
	plan.Tags = mapTagsToModel(builder.Tags, plan.Tags)
	applyResourceTags(ctx, r.client, "Builder", builder.ID, tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BuilderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BuilderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	builder, err := r.client.GetBuilder(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading builder", err.Error())
		return
	}

	state.ID = types.StringValue(builder.ID)
	state.Name = types.StringValue(builder.Name)
	state.ServerID = types.StringValue(builder.Config.ServerID)
	state.Tags = mapTagsToModel(builder.Tags, state.Tags)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BuilderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BuilderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state BuilderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Name.ValueString() != state.Name.ValueString() {
		err := r.client.Write(ctx, "RenameBuilder", map[string]string{
			"id": state.ID.ValueString(), "name": plan.Name.ValueString(),
		}, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error renaming builder", err.Error())
			return
		}
	}

	builder, err := r.client.UpdateBuilder(ctx, client.UpdateBuilderParams{
		ID:     state.ID.ValueString(),
		Config: client.BuilderConfig{ServerID: plan.ServerID.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating builder", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	plan.ID = types.StringValue(builder.ID)
	plan.Name = types.StringValue(builder.Name)
	plan.ServerID = types.StringValue(builder.Config.ServerID)
	plan.Tags = mapTagsToModel(builder.Tags, plan.Tags)
	applyResourceTags(ctx, r.client, "Builder", state.ID.ValueString(), tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BuilderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BuilderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteBuilder(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError("Error deleting builder", err.Error())
	}
}

func (r *BuilderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *BuilderResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || r.client == nil {
		return
	}
	var plan BuilderResourceModel
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
		var priorState BuilderResourceModel
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

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moghtech/terraform-provider-komodo/internal/client"
)

var (
	_ resource.Resource                = &ServerResource{}
	_ resource.ResourceWithImportState = &ServerResource{}
)

type ServerResource struct {
	client *client.Client
}

type ServerResourceModel struct {
	ID                    types.String  `tfsdk:"id"`
	Name                  types.String  `tfsdk:"name"`
	Address               types.String  `tfsdk:"address"`
	ExternalAddress       types.String  `tfsdk:"external_address"`
	Region                types.String  `tfsdk:"region"`
	Enabled               types.Bool    `tfsdk:"enabled"`
	Passkey               types.String  `tfsdk:"passkey"`
	StatsMonitoring       types.Bool    `tfsdk:"stats_monitoring"`
	AutoPrune             types.Bool    `tfsdk:"auto_prune"`
	SendUnreachableAlerts types.Bool    `tfsdk:"send_unreachable_alerts"`
	SendCPUAlerts         types.Bool    `tfsdk:"send_cpu_alerts"`
	SendMemAlerts         types.Bool    `tfsdk:"send_mem_alerts"`
	SendDiskAlerts        types.Bool    `tfsdk:"send_disk_alerts"`
	CPUWarning            types.Float64 `tfsdk:"cpu_warning"`
	CPUCritical           types.Float64 `tfsdk:"cpu_critical"`
	MemWarning            types.Float64 `tfsdk:"mem_warning"`
	MemCritical           types.Float64 `tfsdk:"mem_critical"`
	DiskWarning           types.Float64 `tfsdk:"disk_warning"`
	DiskCritical          types.Float64 `tfsdk:"disk_critical"`
	Tags                  types.List    `tfsdk:"tags"`
}

func NewServerResource() resource.Resource {
	return &ServerResource{}
}

func (r *ServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (r *ServerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Komodo server (a machine running Komodo Periphery).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier of the server.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Display name of the server.",
				Required:    true,
			},
			"address": schema.StringAttribute{
				Description: "HTTP address of the Periphery client (e.g., http://192.168.1.100:8120).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("http://localhost:8120"),
			},
			"external_address": schema.StringAttribute{
				Description: "External address used for container links. If empty, uses the address field.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"region": schema.StringAttribute{
				Description: "Region label for the server.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the server is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"passkey": schema.StringAttribute{
				Description: "Passkey for authenticating with Periphery.",
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
				Default:     stringdefault.StaticString(""),
			},
			"stats_monitoring": schema.BoolAttribute{
				Description: "Whether to monitor server stats beyond health check.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"auto_prune": schema.BoolAttribute{
				Description: "Whether to trigger docker image prune every 24 hours.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"send_unreachable_alerts": schema.BoolAttribute{
				Description: "Whether to send alerts about server reachability.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"send_cpu_alerts": schema.BoolAttribute{
				Description: "Whether to send alerts about CPU status.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"send_mem_alerts": schema.BoolAttribute{
				Description: "Whether to send alerts about memory status.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"send_disk_alerts": schema.BoolAttribute{
				Description: "Whether to send alerts about disk status.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"cpu_warning": schema.Float64Attribute{
				Description: "CPU warning threshold percentage.",
				Optional:    true,
				Computed:    true,
				Default:     float64default.StaticFloat64(90),
			},
			"cpu_critical": schema.Float64Attribute{
				Description: "CPU critical threshold percentage.",
				Optional:    true,
				Computed:    true,
				Default:     float64default.StaticFloat64(99),
			},
			"mem_warning": schema.Float64Attribute{
				Description: "Memory warning threshold percentage.",
				Optional:    true,
				Computed:    true,
				Default:     float64default.StaticFloat64(75),
			},
			"mem_critical": schema.Float64Attribute{
				Description: "Memory critical threshold percentage.",
				Optional:    true,
				Computed:    true,
				Default:     float64default.StaticFloat64(95),
			},
			"disk_warning": schema.Float64Attribute{
				Description: "Disk warning threshold percentage.",
				Optional:    true,
				Computed:    true,
				Default:     float64default.StaticFloat64(75),
			},
			"disk_critical": schema.Float64Attribute{
				Description: "Disk critical threshold percentage.",
				Optional:    true,
				Computed:    true,
				Default:     float64default.StaticFloat64(95),
			},
			"tags": schema.ListAttribute{
				Description: "List of tag names to apply to this server.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *ServerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := client.CreateServerParams{
		Name: plan.Name.ValueString(),
		Config: client.ServerConfig{
			Address:               plan.Address.ValueString(),
			ExternalAddress:       plan.ExternalAddress.ValueString(),
			Region:                plan.Region.ValueString(),
			Enabled:               client.BoolPtr(plan.Enabled.ValueBool()),
			Passkey:               plan.Passkey.ValueString(),
			StatsMonitoring:       client.BoolPtr(plan.StatsMonitoring.ValueBool()),
			AutoPrune:             client.BoolPtr(plan.AutoPrune.ValueBool()),
			SendUnreachableAlerts: client.BoolPtr(plan.SendUnreachableAlerts.ValueBool()),
			SendCPUAlerts:         client.BoolPtr(plan.SendCPUAlerts.ValueBool()),
			SendMemAlerts:         client.BoolPtr(plan.SendMemAlerts.ValueBool()),
			SendDiskAlerts:        client.BoolPtr(plan.SendDiskAlerts.ValueBool()),
			CPUWarning:            client.Float64Ptr(plan.CPUWarning.ValueFloat64()),
			CPUCritical:           client.Float64Ptr(plan.CPUCritical.ValueFloat64()),
			MemWarning:            client.Float64Ptr(plan.MemWarning.ValueFloat64()),
			MemCritical:           client.Float64Ptr(plan.MemCritical.ValueFloat64()),
			DiskWarning:           client.Float64Ptr(plan.DiskWarning.ValueFloat64()),
			DiskCritical:          client.Float64Ptr(plan.DiskCritical.ValueFloat64()),
		},
	}

	server, err := r.client.CreateServer(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error creating server", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	mapServerToModel(server, &plan)
	applyResourceTags(ctx, r.client, "Server", server.ID, tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	server, err := r.client.GetServer(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading server", err.Error())
		return
	}

	mapServerToModel(server, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := client.UpdateServerParams{
		ID: state.ID.ValueString(),
		Config: client.ServerConfig{
			Address:               plan.Address.ValueString(),
			ExternalAddress:       plan.ExternalAddress.ValueString(),
			Region:                plan.Region.ValueString(),
			Enabled:               client.BoolPtr(plan.Enabled.ValueBool()),
			Passkey:               plan.Passkey.ValueString(),
			StatsMonitoring:       client.BoolPtr(plan.StatsMonitoring.ValueBool()),
			AutoPrune:             client.BoolPtr(plan.AutoPrune.ValueBool()),
			SendUnreachableAlerts: client.BoolPtr(plan.SendUnreachableAlerts.ValueBool()),
			SendCPUAlerts:         client.BoolPtr(plan.SendCPUAlerts.ValueBool()),
			SendMemAlerts:         client.BoolPtr(plan.SendMemAlerts.ValueBool()),
			SendDiskAlerts:        client.BoolPtr(plan.SendDiskAlerts.ValueBool()),
			CPUWarning:            client.Float64Ptr(plan.CPUWarning.ValueFloat64()),
			CPUCritical:           client.Float64Ptr(plan.CPUCritical.ValueFloat64()),
			MemWarning:            client.Float64Ptr(plan.MemWarning.ValueFloat64()),
			MemCritical:           client.Float64Ptr(plan.MemCritical.ValueFloat64()),
			DiskWarning:           client.Float64Ptr(plan.DiskWarning.ValueFloat64()),
			DiskCritical:          client.Float64Ptr(plan.DiskCritical.ValueFloat64()),
		},
	}

	// Handle rename if name changed.
	if plan.Name.ValueString() != state.Name.ValueString() {
		err := r.client.Write(ctx, "RenameServer", map[string]string{
			"id":   state.ID.ValueString(),
			"name": plan.Name.ValueString(),
		}, nil)
		if err != nil {
			resp.Diagnostics.AddError("Error renaming server", err.Error())
			return
		}
	}

	server, err := r.client.UpdateServer(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error updating server", err.Error())
		return
	}

	tagsFromPlan := plan.Tags
	mapServerToModel(server, &plan)
	applyResourceTags(ctx, r.client, "Server", state.ID.ValueString(), tagsFromPlan, &resp.Diagnostics)
	if !tagsFromPlan.IsUnknown() {
		plan.Tags = tagsFromPlan
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteServer(ctx, state.ID.ValueString())
	if err != nil {
		var nfe *client.NotFoundError
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError("Error deleting server", err.Error())
	}
}

func (r *ServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapServerToModel(s *client.Server, m *ServerResourceModel) {
	m.ID = types.StringValue(s.ID)
	m.Name = types.StringValue(s.Name)
	m.Address = types.StringValue(s.Config.Address)
	m.ExternalAddress = types.StringValue(s.Config.ExternalAddress)
	m.Region = types.StringValue(s.Config.Region)
	m.Passkey = types.StringValue(s.Config.Passkey)

	if s.Config.Enabled != nil {
		m.Enabled = types.BoolValue(*s.Config.Enabled)
	}
	if s.Config.StatsMonitoring != nil {
		m.StatsMonitoring = types.BoolValue(*s.Config.StatsMonitoring)
	}
	if s.Config.AutoPrune != nil {
		m.AutoPrune = types.BoolValue(*s.Config.AutoPrune)
	}
	if s.Config.SendUnreachableAlerts != nil {
		m.SendUnreachableAlerts = types.BoolValue(*s.Config.SendUnreachableAlerts)
	}
	if s.Config.SendCPUAlerts != nil {
		m.SendCPUAlerts = types.BoolValue(*s.Config.SendCPUAlerts)
	}
	if s.Config.SendMemAlerts != nil {
		m.SendMemAlerts = types.BoolValue(*s.Config.SendMemAlerts)
	}
	if s.Config.SendDiskAlerts != nil {
		m.SendDiskAlerts = types.BoolValue(*s.Config.SendDiskAlerts)
	}
	if s.Config.CPUWarning != nil {
		m.CPUWarning = types.Float64Value(*s.Config.CPUWarning)
	}
	if s.Config.CPUCritical != nil {
		m.CPUCritical = types.Float64Value(*s.Config.CPUCritical)
	}
	if s.Config.MemWarning != nil {
		m.MemWarning = types.Float64Value(*s.Config.MemWarning)
	}
	if s.Config.MemCritical != nil {
		m.MemCritical = types.Float64Value(*s.Config.MemCritical)
	}
	if s.Config.DiskWarning != nil {
		m.DiskWarning = types.Float64Value(*s.Config.DiskWarning)
	}
	if s.Config.DiskCritical != nil {
		m.DiskCritical = types.Float64Value(*s.Config.DiskCritical)
	}

	m.Tags = mapTagsToModel(s.Tags, m.Tags)
}

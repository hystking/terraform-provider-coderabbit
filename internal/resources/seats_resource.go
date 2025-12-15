package resources

import (
	"context"
	"fmt"

	"github.com/coderabbitai/terraform-provider-coderabbit/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource              = &SeatsResource{}
	_ resource.ResourceWithConfigure = &SeatsResource{}
	_ resource.ResourceWithImportState = &SeatsResource{}
)

// SeatsResource defines the resource implementation
type SeatsResource struct {
	client *client.Client
}

// SeatsResourceModel describes the resource data model
type SeatsResourceModel struct {
	ID        types.String `tfsdk:"id"`
	GitHubID  types.String `tfsdk:"github_id"`
	GitUserID types.String `tfsdk:"git_user_id"`
}

// NewSeatsResource creates a new seats resource
func NewSeatsResource() resource.Resource {
	return &SeatsResource{}
}

func (r *SeatsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_seats"
}

func (r *SeatsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a CodeRabbit seat assignment for a user.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier for this resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"github_id": schema.StringAttribute{
				Description: "The GitHub username (e.g., 'octocat'). The provider will automatically resolve this to the numeric git_user_id.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"git_user_id": schema.StringAttribute{
				Description: "The resolved numeric GitHub user ID. This is computed automatically from github_id.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SeatsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *SeatsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SeatsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	githubID := data.GitHubID.ValueString()

	// Resolve GitHub username to numeric user ID
	gitUserID, err := r.client.GetGitUserID(githubID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Resolving GitHub User ID",
			fmt.Sprintf("Could not resolve GitHub username '%s' to numeric ID: %s", githubID, err.Error()),
		)
		return
	}

	// Check if seat is already assigned (idempotency)
	hasSeat, err := r.client.HasSeat(gitUserID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking Seat Assignment",
			fmt.Sprintf("Could not check seat assignment for user %s: %s", githubID, err.Error()),
		)
		return
	}

	if hasSeat {
		// Seat already assigned, just record the state
		tflog.Info(ctx, "Seat already assigned, skipping assign API call", map[string]interface{}{
			"github_id":   githubID,
			"git_user_id": gitUserID,
		})
	} else {
		// Assign seat
		err = r.client.AssignSeat(gitUserID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Assigning Seat",
				fmt.Sprintf("Could not assign seat to user %s (git_user_id: %s): %s", githubID, gitUserID, err.Error()),
			)
			return
		}
		tflog.Info(ctx, "Seat assigned successfully", map[string]interface{}{
			"github_id":   githubID,
			"git_user_id": gitUserID,
		})
	}

	data.ID = types.StringValue(gitUserID)
	data.GitUserID = types.StringValue(gitUserID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SeatsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SeatsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gitUserID := data.GitUserID.ValueString()

	hasSeat, err := r.client.HasSeat(gitUserID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Seat Assignment",
			fmt.Sprintf("Could not read seat assignment for user %s: %s", gitUserID, err.Error()),
		)
		return
	}

	if !hasSeat {
		// Resource no longer exists, remove from state
		tflog.Info(ctx, "Seat not found, removing from state", map[string]interface{}{
			"git_user_id": gitUserID,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SeatsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Since github_id has RequiresReplace, Update should never be called
	// But we implement it for safety
	var data SeatsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SeatsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SeatsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gitUserID := data.GitUserID.ValueString()

	// Check if seat is still assigned before unassigning (idempotency)
	hasSeat, err := r.client.HasSeat(gitUserID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking Seat Assignment",
			fmt.Sprintf("Could not check seat assignment for user %s: %s", gitUserID, err.Error()),
		)
		return
	}

	if !hasSeat {
		// Seat already unassigned, nothing to do
		tflog.Info(ctx, "Seat already unassigned, skipping unassign API call", map[string]interface{}{
			"git_user_id": gitUserID,
		})
		return
	}

	err = r.client.UnassignSeat(gitUserID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Unassigning Seat",
			fmt.Sprintf("Could not unassign seat from user %s: %s", gitUserID, err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Seat unassigned successfully", map[string]interface{}{
		"git_user_id": gitUserID,
	})
}

// ImportState allows importing existing seat assignments
func (r *SeatsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by github_id
	githubID := req.ID

	gitUserID, err := r.client.GetGitUserID(githubID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Seat",
			fmt.Sprintf("Could not resolve GitHub username '%s': %s", githubID, err.Error()),
		)
		return
	}

	// Check if seat exists
	hasSeat, err := r.client.HasSeat(gitUserID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Checking Seat",
			fmt.Sprintf("Could not check seat for user %s: %s", githubID, err.Error()),
		)
		return
	}

	if !hasSeat {
		resp.Diagnostics.AddError(
			"Seat Not Found",
			fmt.Sprintf("User '%s' (git_user_id: %s) does not have a seat assigned", githubID, gitUserID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), gitUserID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("github_id"), githubID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("git_user_id"), gitUserID)...)
}

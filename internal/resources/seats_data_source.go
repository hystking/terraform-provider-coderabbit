package resources

import (
	"context"
	"fmt"

	"github.com/coderabbitai/terraform-provider-coderabbit/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &SeatsDataSource{}
	_ datasource.DataSourceWithConfigure = &SeatsDataSource{}
)

// SeatsDataSource defines the data source implementation
type SeatsDataSource struct {
	client *client.Client
}

// SeatsDataSourceModel describes the data source data model
type SeatsDataSourceModel struct {
	ID                types.String   `tfsdk:"id"`
	UsersWithSeats    []types.String `tfsdk:"users_with_seats"`
	UsersWithoutSeats []types.String `tfsdk:"users_without_seats"`
}

// NewSeatsDataSource creates a new seats data source
func NewSeatsDataSource() datasource.DataSource {
	return &SeatsDataSource{}
}

func (d *SeatsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_seats"
}

func (d *SeatsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves information about CodeRabbit seat assignments.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier for this data source.",
				Computed:    true,
			},
			"users_with_seats": schema.ListAttribute{
				Description: "List of Git user IDs that have seats assigned.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"users_without_seats": schema.ListAttribute{
				Description: "List of Git user IDs that do not have seats assigned.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *SeatsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *SeatsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SeatsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	seats, err := d.client.GetSeats()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Seats",
			fmt.Sprintf("Could not read seat assignments: %s", err.Error()),
		)
		return
	}

	data.ID = types.StringValue("seats")

	// Separate users by seat assignment status
	var usersWithSeats []types.String
	var usersWithoutSeats []types.String

	for _, user := range seats.Users {
		if user.SeatAssigned {
			usersWithSeats = append(usersWithSeats, types.StringValue(user.GitUserID))
		} else {
			usersWithoutSeats = append(usersWithoutSeats, types.StringValue(user.GitUserID))
		}
	}

	data.UsersWithSeats = usersWithSeats
	data.UsersWithoutSeats = usersWithoutSeats

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

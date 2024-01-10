package clients

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/google/uuid"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"
)

// https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles

type Role struct {
	Name string
	// format string with a single %s for the subscription id
	Id string
}

// Roles which must be assigned to provisoned public dns zone, private dns zone, and vnet respectively

var (
	DnsContributorRole = Role{
		Name: "DNS Zone Contributor",
		Id:   "/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/befefa01-2a29-4197-83a8-272ff33ce314",
	}
	PrivateDnsContributorRole = Role{
		Name: "Private DNS Zone Contributor",
		Id:   "/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/b12aa53e-6015-4669-85d0-8515ebb3ae7f",
	}
	NetworkContributorRole = Role{
		Name: "Network Contributor",
		Id:   "/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/b34d265f-36f7-4a0d-a4d4-e158ca92e90f",
	}
)

type roleAssignment struct{}

// Returns a new role assignment to assign to a provisoined infra using the provisioned cluster's principal id
func NewRoleAssignment(ctx context.Context, subscriptionId, scope, principalId string, role Role) (*roleAssignment, error) {
	lgr := logger.FromContext(ctx).With("role", role.Name, "subscriptionId", subscriptionId, "scope", scope, "principalId", principalId)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to create role assignment")
	defer lgr.Info("finished creating role assignment")

	cred, err := GetAzCred()
	if err != nil {
		return nil, fmt.Errorf("getting az credentials: %w", err)
	}

	client, err := armauthorization.NewRoleAssignmentsClient(subscriptionId, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	_, err = client.Create(ctx, scope, uuid.New().String(), armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			RoleDefinitionID: to.Ptr(fmt.Sprintf(role.Id, subscriptionId)),
			PrincipalID:      to.Ptr(principalId),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("creating role assignment: %w", err)
	}

	return &roleAssignment{}, nil
}

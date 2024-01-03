package clients

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerregistry/armcontainerregistry"
	"github.com/Azure/go-autorest/autorest/azure"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"
)

type acr struct {
	id             string
	name           string
	resourceGroup  string
	subscriptionId string
}

func LoadAcr(id azure.Resource) *acr {
	return &acr{
		id:             id.String(),
		name:           id.ResourceName,
		resourceGroup:  id.ResourceGroup,
		subscriptionId: id.SubscriptionID,
	}
}

func NewAcr(ctx context.Context, subscriptionId, resourceGroup, name, location string) (*acr, error) {
	name = nonAlphanumericRegex.ReplaceAllString(name, "")

	lgr := logger.FromContext(ctx).With("name", name, "resourceGroup", resourceGroup, "location", location)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to create acr")
	defer lgr.Info("finished creating acr")

	cred, err := GetAzCred()
	if err != nil {
		return nil, fmt.Errorf("getting az credentials: %w", err)
	}

	factory, err := armcontainerregistry.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating client factory: %w", err)
	}

	r := &armcontainerregistry.Registry{
		Location: to.Ptr(location),
		SKU: &armcontainerregistry.SKU{
			Name: to.Ptr(armcontainerregistry.SKUNameBasic),
		},
	}
	poller, err := factory.NewRegistriesClient().BeginCreate(ctx, resourceGroup, name, *r, nil)
	if err != nil {
		return nil, fmt.Errorf("starting to create registry: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("waiting for registry creation to complete: %w", err)
	}

	// guard against things that should be impossible
	if result.ID == nil {
		return nil, fmt.Errorf("id is nil")
	}
	if result.Name == nil {
		return nil, fmt.Errorf("name is nil")
	}

	return &acr{
		id:             *result.ID,
		name:           *result.Name,
		resourceGroup:  resourceGroup,
		subscriptionId: subscriptionId,
	}, nil
}

func (a *acr) GetName() string {
	return a.name
}

func (a *acr) GetId() string {
	return a.id
}

package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"
)

type rg struct {
	name string
	id   string
}

type RgOpt func(rg *armresources.ResourceGroup) error

// Used for testing purposes: deletes resource group after specified duration
func DeleteAfterOpt(d time.Duration) RgOpt {
	return func(rg *armresources.ResourceGroup) error {
		if rg.Tags == nil {
			rg.Tags = map[string]*string{}
		}

		rg.Tags["deletion_marked_by"] = to.Ptr("gc")
		rg.Tags["deletion_due_time"] = to.Ptr(fmt.Sprint(time.Now().Add(d).Unix()))

		return nil
	}
}

// Called when loading provisioned infrastructure from .json file, returns an rg struct
func LoadRg(id arm.ResourceID) *rg {
	return &rg{
		id:   id.String(),
		name: id.Name,
	}
}

// Creates a new resource group with specified options vis rgOpts. Location used by all resources is specified in infra > infras.go
func NewResourceGroup(ctx context.Context, subscriptionId, name, location string, rgOpts ...RgOpt) (*rg, error) {
	lgr := logger.FromContext(ctx).With("name", name, "location", location, "subscriptionId", subscriptionId)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to create resource group")
	defer lgr.Info("finished creating resource group")

	cred, err := GetAzCred()
	if err != nil {
		return nil, fmt.Errorf("getting az credentials: %w", err)
	}

	client, err := armresources.NewResourceGroupsClient(subscriptionId, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating resource group client: %w", err)
	}

	r := armresources.ResourceGroup{
		Location: to.Ptr(location),
	}
	for _, opt := range rgOpts {
		if err := opt(&r); err != nil {
			return nil, fmt.Errorf("applying resource group option: %w", err)
		}
	}

	resp, err := client.CreateOrUpdate(ctx, name, r, nil)
	if err != nil {
		return nil, fmt.Errorf("creating resource group: %w", err)
	}

	// guard against things that should be impossible
	if resp.ID == nil {
		return nil, fmt.Errorf("resource group ID is nil")
	}
	if resp.Name == nil {
		return nil, fmt.Errorf("resource group name is nil")
	}

	return &rg{
		name: *resp.Name,
		id:   *resp.ID,
	}, nil
}

func (r *rg) GetName() string {
	return r.name
}

func (r *rg) GetId() string {
	return r.id
}

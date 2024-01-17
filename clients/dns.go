package clients

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"github.com/Azure/go-autorest/autorest/azure"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"
)

// zone and private zone types make loading provisioned infra from .json file easier
type zone struct {
	name           string
	subscriptionId string
	resourceGroup  string
	id             string
	nameservers    []string
}

type privateZone struct {
	name           string
	subscriptionId string
	resourceGroup  string
	id             string
}

// ZoneOpt specifies what kind of zone to create
type ZoneOpt func(z *armdns.Zone) error

// PrivateZoneOpt specifies what kind of private zone to create
type PrivateZoneOpt func(z *armprivatedns.PrivateZone) error

// Called to create Provisioned object from .json file
func LoadZone(id azure.Resource, nameservers []string) *zone {
	return &zone{
		id:             id.String(),
		name:           id.ResourceName,
		subscriptionId: id.SubscriptionID,
		resourceGroup:  id.ResourceGroup,
		nameservers:    nameservers,
	}
}

// Returns a private or public zone (based on zoneOpts) with the given name, creates and returns zone struct or an error if one occurred
// comment
func NewZone(ctx context.Context, subscriptionId, resourceGroup, name string, zoneOpts ...ZoneOpt) (*zone, error) {
	name = nonAlphanumericRegex.ReplaceAllString(name, "")
	name = name + ".com"

	lgr := logger.FromContext(ctx).With("name", name, "subscriptionId", subscriptionId, "resourceGroup", resourceGroup)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to create zone")
	defer lgr.Info("finished creating zone")

	cred, err := GetAzCred()
	if err != nil {
		return nil, fmt.Errorf("getting az credentials: %w", err)
	}

	factory, err := armdns.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating client factory: %w", err)
	}

	z := &armdns.Zone{
		Location: to.Ptr("global"), // https://github.com/Azure/azure-cli/issues/6052 this must be global because DNS zones are global resources
		Name:     to.Ptr(name),
	}
	for _, opt := range zoneOpts {
		if err := opt(z); err != nil {
			return nil, fmt.Errorf("applying zone option: %w", err)
		}
	}

	resp, err := factory.NewZonesClient().CreateOrUpdate(ctx, resourceGroup, name, *z, nil)
	if err != nil {
		return nil, fmt.Errorf("creating zone: %w", err)
	}

	// guard against two things that should be impossible
	if resp.Properties == nil {
		return nil, fmt.Errorf("zone properties are nil")
	}
	if resp.Properties.NameServers == nil {
		return nil, fmt.Errorf("zone nameservers are nil")
	}
	if resp.Name == nil {
		return nil, fmt.Errorf("zone name is nil")
	}
	if resp.ID == nil {
		return nil, fmt.Errorf("zone id is nil")
	}

	nameservers := make([]string, len(resp.Properties.NameServers))
	for i, ns := range resp.Properties.NameServers {
		if ns == nil {
			return nil, fmt.Errorf("zone nameserver %d is nil", i)
		}

		nameservers[i] = *ns
	}

	return &zone{
		name:           *resp.Name,
		resourceGroup:  resourceGroup,
		subscriptionId: subscriptionId,
		id:             *resp.ID,
		nameservers:    nameservers,
	}, nil
}

// Returns dns zone from provisioned zone struct, with an error if one occurs
func (z *zone) GetDnsZone(ctx context.Context) (*armdns.Zone, error) {
	lgr := logger.FromContext(ctx).With("name", z.name, "subscriptionId", z.subscriptionId, "resourceGroup", z.resourceGroup)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to get dns")
	defer lgr.Info("finished getting dns")

	cred, err := GetAzCred()
	if err != nil {
		return nil, fmt.Errorf("getting az credentials: %w", err)
	}

	client, err := armdns.NewZonesClient(z.subscriptionId, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	resp, err := client.Get(ctx, z.resourceGroup, z.name, nil)
	if err != nil {
		return nil, fmt.Errorf("getting dns: %w", err)
	}

	return &resp.Zone, nil
}

func (z *zone) GetName() string {
	return z.name
}

func (z *zone) GetNameservers() []string {
	return z.nameservers
}

func (z *zone) GetId() string {
	return z.id
}

// Loads provisioned private zone, used to convert .json saved to infrastructure file to a Provisioned object
func LoadPrivateZone(id azure.Resource) *privateZone {
	return &privateZone{
		id:             id.String(),
		name:           id.ResourceName,
		subscriptionId: id.SubscriptionID,
		resourceGroup:  id.ResourceGroup,
	}
}

// Creates a private zone used in testing
func NewPrivateZone(ctx context.Context, subscriptionId, resourceGroup, name string, opts ...PrivateZoneOpt) (*privateZone, error) {
	name = nonAlphanumericRegex.ReplaceAllString(name, "")
	name = name + ".com"

	lgr := logger.FromContext(ctx).With("name", name, "subscriptionId", subscriptionId, "resourceGroup", resourceGroup)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to create private zone")
	defer lgr.Info("finished creating private zone")

	cred, err := GetAzCred()
	if err != nil {
		return nil, fmt.Errorf("getting az credentials: %w", err)
	}

	client, err := armprivatedns.NewPrivateZonesClient(subscriptionId, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	pz := armprivatedns.PrivateZone{
		Location: to.Ptr("global"),
		Name:     to.Ptr(name),
	}
	for _, opt := range opts {
		if err := opt(&pz); err != nil {
			return nil, fmt.Errorf("applying private zone option: %w", err)
		}
	}
	poller, err := client.BeginCreateOrUpdate(ctx, resourceGroup, name, pz, nil)
	if err != nil {
		return nil, fmt.Errorf("creating private zone: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("creating private zone: %w", err)
	}

	// guard against things that should be impossible
	if result.PrivateZone.Name == nil {
		return nil, fmt.Errorf("private zone name is nil")
	}
	if result.ID == nil {
		return nil, fmt.Errorf("private zone id is nil")
	}

	return &privateZone{
		name:           *result.PrivateZone.Name,
		subscriptionId: subscriptionId,
		resourceGroup:  resourceGroup,
		id:             *result.ID,
	}, nil
}

func (p *privateZone) GetName() string {
	return p.name
}

// Retrieves provisioned Private dns zone
func (p *privateZone) GetDnsZone(ctx context.Context) (*armprivatedns.PrivateZone, error) {
	lgr := logger.FromContext(ctx).With("name", p.name, "subscriptionId", p.subscriptionId, "resourceGroup", p.resourceGroup)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to get private dns")
	defer lgr.Info("finished getting private dns")

	cred, err := GetAzCred()
	if err != nil {
		return nil, fmt.Errorf("getting az credentials: %w", err)
	}

	client, err := armprivatedns.NewPrivateZonesClient(p.subscriptionId, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	resp, err := client.Get(ctx, p.resourceGroup, p.name, nil)
	if err != nil {
		return nil, fmt.Errorf("getting dns: %w", err)
	}

	return &resp.PrivateZone, nil
}

// Creates a virtual network link for new private dns zone
func (p *privateZone) LinkVnet(ctx context.Context, linkName, vnetId string) error {
	linkName = nonAlphanumericRegex.ReplaceAllString(linkName, "")
	linkName = truncate(linkName, 80)

	lgr := logger.FromContext(ctx).With("name", p.name, "subscriptionId", p.subscriptionId, "resourceGroup", p.resourceGroup, "linkName", linkName, "vnetId", vnetId)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to link vnet")
	defer lgr.Info("finished linking vnet")

	cred, err := GetAzCred()
	if err != nil {
		return fmt.Errorf("getting az credentials: %w", err)
	}

	factory, err := armprivatedns.NewClientFactory(p.subscriptionId, cred, nil)
	if err != nil {
		return fmt.Errorf("creating client factory: %w", err)
	}

	l := armprivatedns.VirtualNetworkLink{
		Location: to.Ptr("global"),
		Properties: &armprivatedns.VirtualNetworkLinkProperties{
			RegistrationEnabled: to.Ptr(false),
			VirtualNetwork: &armprivatedns.SubResource{
				ID: to.Ptr(vnetId),
			},
		},
		Name: to.Ptr(linkName),
	}
	_, err = factory.NewVirtualNetworkLinksClient().BeginCreateOrUpdate(ctx, p.resourceGroup, p.name, linkName, l, nil)
	if err != nil {
		return fmt.Errorf("creating virtual network link: %w", err)
	}

	return nil
}

func (p *privateZone) GetId() string {
	return p.id
}

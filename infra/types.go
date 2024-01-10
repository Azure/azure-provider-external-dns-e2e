package infra

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	"github.com/Azure/go-autorest/autorest/azure"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
)

type infras []infra

type infra struct {
	Name   string
	Suffix string
	// ResourceGroup is a unique new resource group name
	// for resources to be provisioned inside
	ResourceGroup, Location string
	McOpts                  []clients.McOpt
}

// McOpt specifies what kind of managed cluster to create
type McOpt struct {
	Name string
	fn   func(mc *armcontainerservice.ManagedCluster) error
}

// PrivateClusterOpt specifies that the cluster should be private
var PrivateClusterOpt = McOpt{
	Name: "private cluster",
	fn: func(mc *armcontainerservice.ManagedCluster) error {
		if mc.Properties == nil {
			mc.Properties = &armcontainerservice.ManagedClusterProperties{}
		}

		if mc.Properties.APIServerAccessProfile == nil {
			mc.Properties.APIServerAccessProfile = &armcontainerservice.ManagedClusterAPIServerAccessProfile{}
		}

		mc.Properties.APIServerAccessProfile.EnablePrivateCluster = to.Ptr(true)
		return nil
	},
}

type Identifier interface {
	GetId() string
}

type cluster interface {
	GetVnetId(ctx context.Context) (string, error)
	Deploy(ctx context.Context, objs []client.Object) error
	GetPrincipalId() string
	GetClientId() string
	GetLocation() string
	GetDnsServiceIp() string
	GetCluster(ctx context.Context) (*armcontainerservice.ManagedCluster, error)
	GetOptions() map[string]struct{}
	Identifier
}

type zone interface {
	GetDnsZone(ctx context.Context) (*armdns.Zone, error)
	GetName() string
	GetNameservers() []string
	Identifier
}

type privateZone interface {
	GetDnsZone(ctx context.Context) (*armprivatedns.PrivateZone, error)
	LinkVnet(ctx context.Context, linkName, vnetId string) error
	GetName() string
	Identifier
}

type resourceGroup interface {
	GetName() string
	Identifier
}

// Provisioned is a struct that contains all the resources provisioned in by the infra command (provision.go)
// Configuration is saved in this struct when reading from infrastructure configuration .json file
type Provisioned struct {
	Name            string
	Cluster         cluster
	ResourceGroup   resourceGroup
	SubscriptionId  string
	TenantId        string
	Zones           []zone
	PrivateZones    []privateZone
	Ipv4ServiceName string
	Ipv6ServiceName string
}

type LoadableZone struct {
	ResourceId  azure.Resource
	Nameservers []string
}

// LoadableProvisioned is a struct that can be used to load a Provisioned struct from a file.
// Ensure that all fields are exported so that they can properly be serialized/deserialized.
type LoadableProvisioned struct {
	Name                                                                      string
	Cluster                                                                   azure.Resource
	ClusterLocation, ClusterDnsServiceIp, ClusterPrincipalId, ClusterClientId string
	ClusterOptions                                                            map[string]struct{}
	ResourceGroup                                                             arm.ResourceID // rg id is a little weird and can't be correctly parsed by azure.Resource so we have to use arm.ResourceID
	SubscriptionId                                                            string
	TenantId                                                                  string
	Zones                                                                     []LoadableZone
	PrivateZones                                                              []azure.Resource
	Ipv4ServiceName                                                           string
	Ipv6ServiceName                                                           string
}

package pkgManifests

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/Azure/azure-provider-external-dns-e2e/pkgResources/config"
)

type configStruct struct {
	Name       string
	Conf       *config.Config
	Deploy     *appsv1.Deployment
	DnsConfigs []*ExternalDnsConfig
}

// Sets public dns configuration above with values from provisioned infra
func GetPublicDnsConfig(tenantId, subId, rg string, publicZone string) *ExternalDnsConfig {

	publicDnsConfig := &ExternalDnsConfig{}
	var publicZonePaths []string
	i := 0

	path := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnszones/%s", subId, rg, publicZone)
	publicZonePaths = append(publicZonePaths, path)
	i++

	publicDnsConfig.TenantId = tenantId
	publicDnsConfig.Subscription = subId
	publicDnsConfig.ResourceGroup = rg
	publicDnsConfig.DnsZoneResourceIDs = publicZonePaths
	publicDnsConfig.Provider = PublicProvider

	return publicDnsConfig

}

// Sets private dns configuration above with values from provisioned infra
func GetPrivateDnsConfig(tenantId, subId, rg string, privateZone string) *ExternalDnsConfig {

	privateDnsConfig := &ExternalDnsConfig{}

	var privateZonePaths []string
	i := 0

	path := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privatednszones/%s", subId, rg, privateZone)
	privateZonePaths = append(privateZonePaths, path)
	i++

	privateDnsConfig.TenantId = tenantId
	privateDnsConfig.Subscription = subId
	privateDnsConfig.ResourceGroup = rg
	privateDnsConfig.DnsZoneResourceIDs = privateZonePaths
	privateDnsConfig.Provider = PrivateProvider

	return privateDnsConfig
}

// Initializes Example configuration with public and private dns config. Called from Provision.go
func SetExampleConfig(clientId, clusterUid string, publicDnsConfig, privateDnsConfig *ExternalDnsConfig) []configStruct {
	//for now, we have one configuration, returning an array of configStructs allows us to rotate between configs if necessary
	exampleConfigs := []configStruct{
		{
			Name:       "full",
			Conf:       &config.Config{NS: "kube-system", MSIClientID: clientId, ClusterUid: clusterUid, DnsSyncInterval: time.Minute * 3, Registry: "mcr.microsoft.com"},
			Deploy:     nil,
			DnsConfigs: []*ExternalDnsConfig{publicDnsConfig, privateDnsConfig},
		},
		//add other configs here
	}

	return exampleConfigs

}

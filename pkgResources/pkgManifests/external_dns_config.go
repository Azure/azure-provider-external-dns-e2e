package pkgManifests

import (
	"fmt"
	"time"

	"github.com/Azure/azure-provider-external-dns-e2e/pkgResources/config"
	appsv1 "k8s.io/api/apps/v1"
)

type configStruct struct {
	Name       string
	Conf       *config.Config
	Deploy     *appsv1.Deployment
	DnsConfigs []*ExternalDnsConfig
}

// Sets public dns configuration above with values from provisioned infra
func GetPublicDnsConfig(tenantId, subId, rg string, publicZones []string) *ExternalDnsConfig {

	publicDnsConfig := &ExternalDnsConfig{}

	fmt.Println("Setting configuration for ext dns")
	var publicZonePaths []string
	i := 0

	for i < len(publicZones) {
		path := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnszones/%s", subId, rg, publicZones[i])
		publicZonePaths = append(publicZonePaths, path)
		i++
	}

	publicDnsConfig.TenantId = tenantId
	publicDnsConfig.Subscription = subId
	publicDnsConfig.ResourceGroup = rg
	publicDnsConfig.DnsZoneResourceIDs = publicZonePaths
	publicDnsConfig.Provider = PublicProvider

	return publicDnsConfig

}

// Sets private dns configuration above with values from provisioned infra
func GetPrivateDnsConfig(tenantId, subId, rg string, privateZones []string) *ExternalDnsConfig {

	privateDnsConfig := &ExternalDnsConfig{}

	fmt.Println("Setting configuration for ext dns")
	var privateZonePaths []string
	i := 0
	for i < len(privateZones) {
		path := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/privatednszones/%s", subId, rg, privateZones[i])
		privateZonePaths = append(privateZonePaths, path)
		i++
	}

	privateDnsConfig.TenantId = tenantId
	privateDnsConfig.Subscription = subId
	privateDnsConfig.ResourceGroup = rg
	privateDnsConfig.DnsZoneResourceIDs = privateZonePaths
	privateDnsConfig.Provider = PrivateProvider

	return privateDnsConfig
}

func SetExampleConfig(clusterUid string, publicDnsConfig, privateDnsConfig *ExternalDnsConfig) []configStruct {
	//for now, we have one configuration, returning an array of configStructs allows us to rotate between configs if necessary
	exampleConfigs := []configStruct{
		{
			Name:       "full",
			Conf:       &config.Config{NS: "kube-system", ClusterUid: clusterUid, DnsSyncInterval: time.Minute * 3, Registry: "mcr.microsoft.com"},
			Deploy:     nil,
			DnsConfigs: []*ExternalDnsConfig{publicDnsConfig, privateDnsConfig},
		},
		//add other configs here
	}

	return exampleConfigs

}

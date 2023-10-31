package pkgManifests

import (
	"time"

	"github.com/Azure/azure-provider-external-dns-e2e/pkgResources/config"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type configStruct struct {
	Name       string
	Conf       *config.Config
	Deploy     *appsv1.Deployment
	DnsConfigs []*ExternalDnsConfig
}

var (
	publicZoneOne = "/subscriptions/test-subscription/resourceGroups/test-rg-private/providers/Microsoft.Network/dnszones/test-one.com"
	publicZoneTwo = "/subscriptions/test-subscription/resourceGroups/test-rg-private/providers/Microsoft.Network/dnszones/test-two.com"
	publicZones   = []string{publicZoneOne, publicZoneTwo}

	privateZoneOne = "/subscriptions/test-subscription/resourceGroups/test-rg-private/providers/Microsoft.Network/privatednszones/test-three.com"
	privateZoneTwo = "/subscriptions/test-subscription/resourceGroups/test-rg-private/providers/Microsoft.Network/privatednszones/test-four.com"
	privateZones   = []string{privateZoneOne, privateZoneTwo}

	clusterUid = "test-cluster-uid"

	publicDnsConfig = &ExternalDnsConfig{
		TenantId:           "test-tenant-id",
		Subscription:       "test-subscription-id",
		ResourceGroup:      "test-resource-group-public",
		DnsZoneResourceIDs: publicZones,
		Provider:           PublicProvider,
	}

	privateDnsConfig = &ExternalDnsConfig{
		TenantId:           "test-tenant-id",
		Subscription:       "test-subscription-id",
		ResourceGroup:      "test-resource-group-private",
		DnsZoneResourceIDs: privateZones,
		Provider:           PrivateProvider,
	}

	exampleConfigs = []configStruct{
		{
			Name:       "full",
			Conf:       &config.Config{NS: "kube-system", ClusterUid: clusterUid, DnsSyncInterval: time.Minute * 3, Registry: "mcr.microsoft.com"},
			Deploy:     nil,
			DnsConfigs: []*ExternalDnsConfig{publicDnsConfig, privateDnsConfig},
		},
		{
			Name:       "no-ownership",
			Conf:       &config.Config{NS: "test-namespace", ClusterUid: clusterUid, DnsSyncInterval: time.Minute * 3},
			DnsConfigs: []*ExternalDnsConfig{publicDnsConfig},
		},
		{
			Name: "private",
			Conf: &config.Config{NS: "test-namespace", ClusterUid: clusterUid, DnsSyncInterval: time.Minute * 3},
			Deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-operator-deploy",
					UID:  "test-operator-deploy-uid",
				},
			},
			DnsConfigs: []*ExternalDnsConfig{privateDnsConfig},
		},
		{
			Name: "short-sync-interval",
			Conf: &config.Config{NS: "test-namespace", ClusterUid: clusterUid, DnsSyncInterval: time.Second * 10},
			Deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-operator-deploy",
					UID:  "test-operator-deploy-uid",
				},
			},
			DnsConfigs: []*ExternalDnsConfig{publicDnsConfig, privateDnsConfig},
		},
	}
)

func GetExampleConfigs() []configStruct {

	return exampleConfigs

}

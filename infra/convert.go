package infra

import (
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/go-autorest/autorest/azure"
)

func (p Provisioned) Loadable() (LoadableProvisioned, error) {
	cluster, err := azure.ParseResourceID(p.Cluster.GetId())
	if err != nil {
		return LoadableProvisioned{}, fmt.Errorf("parsing cluster resource id: %w", err)
	}

	resourceGroup, err := arm.ParseResourceID(p.ResourceGroup.GetId())
	if err != nil {
		return LoadableProvisioned{}, fmt.Errorf("parsing resource group resource id: %w", err)
	}

	return LoadableProvisioned{
		Name:                p.Name,
		Cluster:             cluster,
		ClusterLocation:     p.Cluster.GetLocation(),
		ClusterDnsServiceIp: p.Cluster.GetDnsServiceIp(),
		ClusterPrincipalId:  p.Cluster.GetPrincipalId(),
		ClusterClientId:     p.Cluster.GetClientId(),
		ClusterOptions:      p.Cluster.GetOptions(),
		ResourceGroup:       *resourceGroup,
		SubscriptionId:      p.SubscriptionId,
		TenantId:            p.TenantId,
	}, nil
}

func ToLoadable(p []Provisioned) ([]LoadableProvisioned, error) {
	ret := make([]LoadableProvisioned, len(p))
	for i, provisioned := range p {
		loadable, err := provisioned.Loadable()
		if err != nil {
			return nil, fmt.Errorf("loading provisioned %s: %w", provisioned.Name, err)
		}
		ret[i] = loadable
	}
	return ret, nil
}

func ToProvisioned(l []LoadableProvisioned) ([]Provisioned, error) {
	ret := make([]Provisioned, len(l))
	for i, loadable := range l {
		provisioned, err := loadable.Provisioned()
		if err != nil {
			return nil, fmt.Errorf("parsing loadable %s: %w", loadable.Name, err)
		}
		ret[i] = provisioned
	}
	return ret, nil
}

func (l LoadableProvisioned) Provisioned() (Provisioned, error) {

	return Provisioned{
		Name:           l.Name,
		Cluster:        clients.LoadAks(l.Cluster, l.ClusterDnsServiceIp, l.ClusterLocation, l.ClusterPrincipalId, l.ClusterClientId, l.ClusterOptions),
		ResourceGroup:  clients.LoadRg(l.ResourceGroup),
		SubscriptionId: l.SubscriptionId,
		TenantId:       l.TenantId,
	}, nil
}

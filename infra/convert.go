package infra

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/go-autorest/autorest/azure"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
)

// Used to save provisioned infrastructure to .json file, used by ToLoadable() and called from the infra command
func (p Provisioned) Loadable() (LoadableProvisioned, error) {
	cluster, err := azure.ParseResourceID(p.Cluster.GetId())
	if err != nil {
		return LoadableProvisioned{}, fmt.Errorf("parsing cluster resource id: %w", err)
	}

	resourceGroup, err := arm.ParseResourceID(p.ResourceGroup.GetId())
	if err != nil {
		return LoadableProvisioned{}, fmt.Errorf("parsing resource group resource id: %w", err)
	}

	zones := make([]LoadableZone, len(p.Zones))
	for i, zone := range p.Zones {
		z, err := azure.ParseResourceID(zone.GetId())
		if err != nil {
			return LoadableProvisioned{}, fmt.Errorf("parsing zone resource id: %w", err)
		}
		zones[i] = LoadableZone{
			ResourceId:  z,
			Nameservers: zone.GetNameservers(),
		}
	}

	privateZones := make([]azure.Resource, len(p.PrivateZones))
	for i, privateZone := range p.PrivateZones {
		z, err := azure.ParseResourceID(privateZone.GetId())
		if err != nil {
			return LoadableProvisioned{}, fmt.Errorf("parsing private zone resource id: %w", err)
		}
		privateZones[i] = z
	}

	return LoadableProvisioned{
		Name:                p.Name,
		Cluster:             cluster,
		ClusterLocation:     p.Cluster.GetLocation(),
		ClusterDnsServiceIp: p.Cluster.GetDnsServiceIp(),
		ClusterPrincipalId:  p.Cluster.GetPrincipalId(),
		ClusterClientId:     p.Cluster.GetClientId(),
		ClusterOptions:      p.Cluster.GetOptions(),
		Zones:               zones,
		PrivateZones:        privateZones,
		ResourceGroup:       *resourceGroup,
		SubscriptionId:      p.SubscriptionId,
		TenantId:            p.TenantId,
		Ipv4ServiceName:     p.Ipv4ServiceName,
		Ipv6ServiceName:     p.Ipv6ServiceName,
	}, nil

}

// Returns LoadableProvisioned struct to be saved to infrastructure .json file
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

// Loads Provisioned struct from infrastructure .json file
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

// Used to load Provisioned struct from infra-config.json or other specified infrastructure .json file
func (l LoadableProvisioned) Provisioned() (Provisioned, error) {

	zs := make([]zone, len(l.Zones))
	for i, z := range l.Zones {
		zs[i] = clients.LoadZone(z.ResourceId, z.Nameservers)
	}
	pzs := make([]privateZone, len(l.PrivateZones))
	for i, pz := range l.PrivateZones {
		pzs[i] = clients.LoadPrivateZone(pz)
	}

	return Provisioned{
		Name:            l.Name,
		Cluster:         clients.LoadAks(l.Cluster, l.ClusterDnsServiceIp, l.ClusterLocation, l.ClusterPrincipalId, l.ClusterClientId, l.ClusterOptions),
		Zones:           zs,
		PrivateZones:    pzs,
		ResourceGroup:   clients.LoadRg(l.ResourceGroup),
		SubscriptionId:  l.SubscriptionId,
		TenantId:        l.TenantId,
		Ipv4ServiceName: l.Ipv4ServiceName,
		Ipv6ServiceName: l.Ipv6ServiceName,
	}, nil
}

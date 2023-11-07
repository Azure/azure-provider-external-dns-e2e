package infra

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"

	manifests "github.com/Azure/azure-provider-external-dns-e2e/pkgResources/pkgManifests"
)

const (
	// lenZones is the number of zones to provision
	lenZones = 1
	// lenPrivateZones is the number of private zones to provision
	lenPrivateZones = 1
)

var (
	self *appsv1.Deployment = nil
)

func (i *infra) Provision(ctx context.Context, tenantId, subscriptionId string) (Provisioned, *logger.LoggedError) {
	lgr := logger.FromContext(ctx).With("infra", i.Name)
	lgr.Info("provisioning infrastructure")
	defer lgr.Info("finished provisioning infrastructure")

	ret := Provisioned{
		Name:           i.Name,
		SubscriptionId: subscriptionId,
		TenantId:       tenantId,
	}

	var err error
	ret.ResourceGroup, err = clients.NewResourceGroup(ctx, subscriptionId, i.ResourceGroup, i.Location, clients.DeleteAfterOpt(2*time.Hour))

	if err != nil {
		return Provisioned{}, logger.Error(lgr, fmt.Errorf("creating resource group %s: %w", i.ResourceGroup, err))
	}

	// create resources
	var resEg errgroup.Group

	var subnetId string
	var vnetId string

	//create vnet
	resEg.Go(func() error {
		vnetId, subnetId, err = clients.NewVnet(ctx, subscriptionId, i.ResourceGroup, i.Location)
		if err != nil {
			return logger.Error(lgr, fmt.Errorf("creating vnet: %w", err))
		}
		return nil
	})

	if err := resEg.Wait(); err != nil {
		return Provisioned{}, logger.Error(lgr, err)
	}

	resEg.Go(func() error {
		ret.Cluster, err = clients.NewAks(ctx, subscriptionId, i.ResourceGroup, "cluster"+i.Suffix, i.Location, subnetId, i.McOpts...)

		if err != nil {
			return logger.Error(lgr, fmt.Errorf("creating managed cluster: %w", err))
		}

		return nil
	})

	//Add dns zone resource- Currently creating 1 private zone and 1 public zone
	for idx := 0; idx < lenZones; idx++ {
		func(idx int) {
			resEg.Go(func() error {
				zone, err := clients.NewZone(ctx, subscriptionId, i.ResourceGroup, publicZoneName)
				if err != nil {
					return logger.Error(lgr, fmt.Errorf("creating zone: %w", err))
				}
				ret.Zones = append(ret.Zones, zone)
				return nil
			})
		}(idx)
	}
	for idx := 0; idx < lenPrivateZones; idx++ {
		func(idx int) {
			resEg.Go(func() error {
				privateZone, err := clients.NewPrivateZone(ctx, subscriptionId, i.ResourceGroup, privateZoneName)
				if err != nil {
					return logger.Error(lgr, fmt.Errorf("creating private zone: %w", err))
				}
				ret.PrivateZones = append(ret.PrivateZones, privateZone)
				return nil
			})
		}(idx)
	}

	if err := resEg.Wait(); err != nil {
		return Provisioned{}, logger.Error(lgr, err)
	}

	//setting permissions for private zones
	var permEg errgroup.Group
	for _, pz := range ret.PrivateZones {
		func(pz privateZone) {
			permEg.Go(func() error {
				dns, err := pz.GetDnsZone(ctx)
				if err != nil {
					return logger.Error(lgr, fmt.Errorf("getting dns: %w", err))
				}

				principalId := ret.Cluster.GetPrincipalId()
				role := clients.PrivateDnsContributorRole
				if _, err := clients.NewRoleAssignment(ctx, subscriptionId, *dns.ID, principalId, role); err != nil {
					return logger.Error(lgr, fmt.Errorf("creating %s role assignment: %w", role.Name, err))
				}

				return nil
			})
		}(pz)
	}

	//setting permissions for public zones
	for _, z := range ret.Zones {
		func(z zone) {
			permEg.Go(func() error {
				dns, err := z.GetDnsZone(ctx)
				if err != nil {
					return logger.Error(lgr, fmt.Errorf("getting dns: %w", err))
				}

				principalId := ret.Cluster.GetPrincipalId()
				role := clients.DnsContributorRole
				if _, err := clients.NewRoleAssignment(ctx, subscriptionId, *dns.ID, principalId, role); err != nil {
					return logger.Error(lgr, fmt.Errorf("creating %s role assignment: %w", role.Name, err))
				}

				return nil
			})
		}(z)
	}

	//az role assignment create --assignee <control-plane-identity-principal-id> --scope $VNET_ID --role "Network Contributor"
	permEg.Go(func() error {
		principalId := ret.Cluster.GetPrincipalId()

		//TODO: role for subnet, vnet, or both?
		if vnetId == "" || subnetId == "" {
			return logger.Error(lgr, fmt.Errorf("vnet id is empty before role assignment"))
		}

		//Adding network contributor role on the vnet
		role := clients.NetworkContributorRole
		if _, err := clients.NewRoleAssignment(ctx, subscriptionId, vnetId, principalId, role); err != nil {
			return logger.Error(lgr, fmt.Errorf("creating %s role assignment: %w", role.Name, err))
		}

		//Adding network contributor role on the subnet
		if _, err := clients.NewRoleAssignment(ctx, subscriptionId, subnetId, principalId, role); err != nil {
			return logger.Error(lgr, fmt.Errorf("creating %s role assignment: %w", role.Name, err))
		}
		return nil
	})

	if err := permEg.Wait(); err != nil {
		return Provisioned{}, logger.Error(lgr, err)
	}

	//TODO: put in errgroups?
	//Deploy external dns
	err = deployExternalDNS(ctx, ret)
	if err != nil {
		return ret, logger.Error(lgr, fmt.Errorf("error deploying external dns onto cluster %w", err))
	}

	//Create Nginx service -- TODO: remove this, we'll provision the service in the test only
	_, serviceObj, err := deployNginx(ctx, ret)
	if err != nil {
		return ret, logger.Error(lgr, fmt.Errorf("error deploying nginx onto cluster %w", err))
	}

	ret.ServiceName = serviceObj.Name

	// fmt.Println("load balancer: ", serviceObj.Spec.LoadBalancerIP)
	// fmt.Println("Service Name: ", svcInfo.GetName())
	// fmt.Println("Service Ip: ", svcInfo.GetIpAddr())
	// ret.ServiceName = svcInfo.GetName()
	// ret.ServiceIP = svcInfo.GetIpAddr()

	return ret, nil
}

func (is infras) Provision(tenantId, subscriptionId string) ([]Provisioned, error) {
	lgr := logger.FromContext(context.Background())

	lgr.Info("starting to provision all infrastructure")
	defer lgr.Info("finished provisioning all infrastructure")

	var eg errgroup.Group
	provisioned := make([]Provisioned, len(is))

	for idx, inf := range is {
		func(idx int, inf infra) {
			eg.Go(func() error {
				ctx := context.Background()
				lgr := logger.FromContext(ctx)
				ctx = logger.WithContext(ctx, lgr.With("infra", inf.Name))

				provisionedInfra, err := inf.Provision(ctx, tenantId, subscriptionId)
				if err != nil {
					return fmt.Errorf("provisioning infrastructure %s: %w", inf.Name, err)
				}

				provisioned[idx] = provisionedInfra
				return nil
			})
		}(idx, inf)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return provisioned, nil
}

// Creates Nginx deployment and service for testing
func deployNginx(ctx context.Context, p Provisioned) (*clients.SvcInfo, *corev1.Service, error) {

	var objs []client.Object

	lgr := logger.FromContext(ctx).With("infra", p.Name)
	lgr.Info("deploying nginx deployment and service onto cluster")
	defer lgr.Info("finished deploying nginx resources")

	nginxDeployment := clients.NewNginxDeployment()
	svcInfo, nginxService := clients.NewNginxService()
	objs = append(objs, nginxDeployment)
	objs = append(objs, nginxService)

	if err := p.Cluster.Deploy(ctx, objs); err != nil {
		fmt.Println("Error Deploying Nginx resources")
		return svcInfo, nginxService, logger.Error(lgr, err)
	}

	return svcInfo, nginxService, nil

}

// Deploys ExternalDNS onto cluster
func deployExternalDNS(ctx context.Context, p Provisioned) error {

	lgr := logger.FromContext(ctx).With("infra", p.Name)
	lgr.Info("deploying external DNS onto cluster")
	defer lgr.Info("finished deploying ext DNS")

	var publicZoneNames []string
	zones := p.Zones
	i := 0
	for i < lenZones {
		publicZoneNames = append(publicZoneNames, zones[i].GetName())
		i++
	}

	var privateZoneNames []string
	privateZones := p.PrivateZones
	i = 0
	for i < lenZones {
		privateZoneNames = append(privateZoneNames, privateZones[i].GetName())
		i++
	}

	if len(publicZoneNames) == 0 || len(privateZoneNames) == 0 {
		return fmt.Errorf("public and/or private zones were not created/provided")
	}

	fmt.Println("Public zone names: ", publicZoneNames)
	fmt.Println("Private zone names: ", privateZoneNames)

	//TODO: is this the correct Tenant ID?
	publicDnsConfig := manifests.GetPublicDnsConfig(p.TenantId, p.SubscriptionId, p.ResourceGroup.GetName(), publicZoneNames)
	privateDnsConfig := manifests.GetPrivateDnsConfig(p.TenantId, p.SubscriptionId, p.ResourceGroup.GetName(), privateZoneNames)

	//TODO: correct value for cluster uid?
	exConfig := manifests.SetExampleConfig(p.Cluster.GetClientId(), p.Cluster.GetId(), publicDnsConfig, privateDnsConfig)
	currentConfig := exConfig[0] //currently only using one config from external_dns_config.go

	objs := manifests.ExternalDnsResources(currentConfig.Conf, currentConfig.Deploy, currentConfig.DnsConfigs)

	if err := p.Cluster.Deploy(ctx, objs); err != nil {
		fmt.Println("Error Deploying External DNS")
		return logger.Error(lgr, err)
	}

	return nil

}

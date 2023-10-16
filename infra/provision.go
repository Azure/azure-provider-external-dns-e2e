package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	manifests "github.com/Azure/azure-provider-external-dns-e2e/pkgResources/pkgManifests"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/rest"
)

const (
	// lenZones is the number of zones to provision
	lenZones = 2
	// lenPrivateZones is the number of private zones to provision
	lenPrivateZones = 2
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

	resEg.Go(func() error {
		ret.Cluster, err = clients.NewAks(ctx, subscriptionId, i.ResourceGroup, "cluster"+i.Suffix, i.Location, i.McOpts...)
		if err != nil {
			return logger.Error(lgr, fmt.Errorf("creating managed cluster: %w", err))
		}

		return nil
	})

	if err := resEg.Wait(); err != nil {
		return Provisioned{}, logger.Error(lgr, err)
	}

	//Add dns zone resource- private and public
	for idx := 0; idx < lenZones; idx++ {
		func(idx int) {
			resEg.Go(func() error {
				zone, err := clients.NewZone(ctx, subscriptionId, i.ResourceGroup, fmt.Sprintf("zone-%d-%s", idx, i.Suffix))
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
				privateZone, err := clients.NewPrivateZone(ctx, subscriptionId, i.ResourceGroup, fmt.Sprintf("private-zone-%d-%s", idx, i.Suffix))
				if err != nil {
					return logger.Error(lgr, fmt.Errorf("creating private zone: %w", err))
				}
				ret.PrivateZones = append(ret.PrivateZones, privateZone)
				return nil
			})
		}(idx)
	}

	resEg.Go(func() error {
		ret.ContainerRegistry, err = clients.NewAcr(ctx, subscriptionId, i.ResourceGroup, "registry"+i.Suffix, i.Location)
		if err != nil {
			return logger.Error(lgr, fmt.Errorf("creating container registry: %w", err))
		}

		resEg.Go(func() error {
			e2eRepoAndTag := "e2e:" + i.Suffix
			if err := ret.ContainerRegistry.BuildAndPush(ctx, e2eRepoAndTag, "."); err != nil {
				return logger.Error(lgr, fmt.Errorf("building and pushing e2e image: %w", err))
			}
			ret.E2eImage = ret.ContainerRegistry.GetName() + ".azurecr.io/" + e2eRepoAndTag
			return nil
		})

		return nil
	})

	//Deploy external dns
	err = deployExternalDNS(ctx, ret)
	if err != nil {
		logger.Error(lgr, fmt.Errorf("error deploying external dns onto cluster %w", err))
	}

	return ret, nil
} //END OF PROVISION

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

var restConfig *rest.Config

// deploys external dns
func deployExternalDNS(ctx context.Context, p Provisioned) error {

	lgr := logger.FromContext(ctx).With("infra", p.Name)
	lgr.Info("deploying external DNS onto cluster")
	defer lgr.Info("finished deploying ext DNS")

	exConfig := manifests.GetExampleConfigs()[0]

	objs := manifests.ExternalDnsResources(exConfig.Conf, exConfig.Deploy, exConfig.DnsConfigs)

	if err := p.Cluster.Deploy(ctx, objs); err != nil {
		fmt.Println("Error Deploying External DNS")
		return logger.Error(lgr, err)
	}

	return nil

}

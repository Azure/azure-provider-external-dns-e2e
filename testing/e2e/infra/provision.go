package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-provider-external-dns-e2e/testing/e2e/clients"

	"github.com/Azure/azure-provider-external-dns-e2e/testing/e2e/logger"
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
	self      *appsv1.Deployment = nil
	clusterID string
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
	// resEg.Go(func() error {
	// 	ret.ContainerRegistry, err = clients.NewAcr(ctx, subscriptionId, i.ResourceGroup, "registry"+i.Suffix, i.Location)
	// 	if err != nil {
	// 		return logger.Error(lgr, fmt.Errorf("creating container registry: %w", err))
	// 	}

	// 	resEg.Go(func() error {
	// 		e2eRepoAndTag := "e2e:" + i.Suffix
	// 		if err := ret.ContainerRegistry.BuildAndPush(ctx, e2eRepoAndTag, "."); err != nil {
	// 			return logger.Error(lgr, fmt.Errorf("building and pushing e2e image: %w", err))
	// 		}
	// 		ret.E2eImage = ret.ContainerRegistry.GetName() + ".azurecr.io/" + e2eRepoAndTag
	// 		return nil
	// 	})

	// 	resEg.Go(func() error {
	// 		operatorRepoAndTag := "operator:" + i.Suffix
	// 		if err := ret.ContainerRegistry.BuildAndPush(ctx, operatorRepoAndTag, "../../"); err != nil {
	// 			return logger.Error(lgr, fmt.Errorf("building and pushing operator image: %w", err))
	// 		}
	// 		ret.OperatorImage = ret.ContainerRegistry.GetName() + ".azurecr.io/" + operatorRepoAndTag

	// 		return nil
	// 	})

	// 	return nil
	// })

	resEg.Go(func() error {
		ret.Cluster, err = clients.NewAks(ctx, subscriptionId, i.ResourceGroup, "cluster"+i.Suffix, i.Location, i.McOpts...)
		if err != nil {
			return logger.Error(lgr, fmt.Errorf("creating managed cluster: %w", err))
		}
		clusterID = ret.Cluster.GetId()

		return nil
	})

	if err := resEg.Wait(); err != nil {
		return Provisioned{}, logger.Error(lgr, err)
	}

	resEg.Go(func() error {
		deployExternalDNS(clusterID)
		if err != nil {
			return logger.Error(lgr, fmt.Errorf("error deploying external dns onto cluster %w", err))
		}
		return nil
	})

	return ret, nil
} //END OF PROVISION

//deploying external dns -- pass in cluster uid for conf?

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

func deployExternalDNS(clusterId string) error {

	fmt.Println("In deploy external dns >>>>>>>>>>>>>>>>>>>>")
	// m, err := manager.New(restConfig, manager.Options{Metrics: metricsserver.Options{BindAddress: ":0"}})
	// if err != nil {
	// 	fmt.Println("error creating new manager for external dns: ", err)
	// 	return err
	// }

	//replace this with just cluster uid?
	// conf := &config.Config{ClusterUid: clusterId}
	// //conf := &config.Config{NS: "app-routing-system", OperatorDeployment: "operator"}

	// err = extDNS.NewExternalDns(m, conf, self)
	// if err != nil {
	// 	fmt.Println("error creating external dns: ", err)
	// 	return err
	// }
	// require.NoError(t, err)
	return nil

}

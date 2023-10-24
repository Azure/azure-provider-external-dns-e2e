package suites

import (
	"context"
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

var (
	// basicNs is a map of namespaces that are used by the basic suite.
	// the key is the dns zone name, and the value is the namespace that
	// is used for the tests for that dns zone. Using shared namespaces
	// allow us to appropriately test upgrade scenarios.
	basicNs = make(map[string]*corev1.Namespace)
)

func basicSuite(in infra.Provisioned) []test {
	return []test{

		{
			name: "basic service",
			run: func(ctx context.Context, config *rest.Config) error {
				//get provisioned nginx service
				//provisionedService := in.Service

				//remove ingress param from modifier func, maybe zoner too?
				if err := basicRecordTest(ctx, config, basicNs, in, func(service *corev1.Service) error {

					//annotate service. Use infra Service in the function
					annotations := service.GetAnnotations()

					//annotations["kubernetes.azure.com/ingress-host"] = z.GetNameserver()
					//annotations["kubernetes.azure.com/tls-cert-keyvault-uri"] = in.Cert.GetId()
					service.SetAnnotations(annotations)

					return nil
				}); err != nil {
					return err
				}

				return nil
			},
		},
	}
}

// modifier is a function that can be used to modify the ingress and service
type modifier func(service *corev1.Service) error

// Basic public cluster test which tests external dns with a public dns zone and service annotated with an A record
var basicRecordTest = func(ctx context.Context, config *rest.Config, namespaces map[string]*corev1.Namespace, infra infra.Provisioned, mod modifier) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	//$ az network dns record-set a list --resource-group "MyDnsResourceGroup" --zone-name example.com
	// var zones []zoner
	// z := infra.Zones[0]
	//zones = append(zones, zone{name: z.GetName(), nameserver: z.GetNameservers()[0]})

	var eg errgroup.Group
	// for _, zone := range zones {
	// 	zone := zone
	// 	eg.Go(func() error {
	// 		lgr := logger.FromContext(ctx).With("zone", zone.GetName())
	// 		ctx := logger.WithContext(ctx, lgr)

	// 		if val, ok := namespaces[zone.GetName()]; !ok || val == nil {
	// 			namespaces[zone.GetName()] = manifests.UncollisionedNs()
	// 		}
	// 		ns := namespaces[zone.GetName()]

	// 		lgr.Info("creating namespace")
	// 		if err := upsert(ctx, c, ns); err != nil {
	// 			return fmt.Errorf("upserting ns: %w", err)
	// 		}

	// 		lgr = lgr.With("namespace", ns.Name)
	// 		ctx = logger.WithContext(ctx, lgr)

	// 		testingResources := manifests.ClientAndServer(ns.Name, "e2e-testing", zone.GetName(), zone.GetNameserver(), infra.Cert.GetId())
	// 		for _, object := range testingResources.Objects() {
	// 			if err := upsert(ctx, c, object); err != nil {
	// 				return fmt.Errorf("upserting resource: %w", err)
	// 			}
	// 		}

	// 		if mod != nil {
	// 			if err := mod(testingResources.Ingress, testingResources.Service, zone); err != nil {
	// 				return fmt.Errorf("modifying ingress and service: %w", err)
	// 			}
	// 		}

	// 		ctx = logger.WithContext(ctx, lgr.With("client", testingResources.Client.GetName(), "clientNamespace", testingResources.Client.GetNamespace()))
	// 		if err := waitForAvailable(ctx, c, *testingResources.Client); err != nil {
	// 			return fmt.Errorf("waiting for client deployment to be available: %w", err)
	// 		}

	// 		lgr.Info("finished testing zone")
	// 		return nil
	// 	})
	// }

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("testing all zones: %w", err)
	}

	lgr.Info("finished successfully")
	return nil
}

type zoner interface {
	GetName() string
	GetNameserver() string
}

type zone struct {
	name       string
	nameserver string
}

func (z zone) GetName() string {
	return z.name
}

func (z zone) GetNameserver() string {
	return z.nameserver
}

package tests

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	corev1 "k8s.io/api/core/v1"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// global exported vars used by tests
var (
	ClusterName    *string
	Service        *corev1.Service
	SubscriptionId string
	ResourceGroup  string
)

func init() {
	log.SetLogger(logr.New(log.NullLogSink{})) // without this controller-runtime panics. We use it solely for the client so we can ignore logs

}

func (allTests Ts) Run(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("In All tests RUN FUNCTION >>>>>>>>>>>>>>>>>>>>>>>>>>")

	SubscriptionId = infra.SubscriptionId
	ResourceGroup = infra.ResourceGroup.GetName()
	//setting exported vars used by all tests
	cluster, err := infra.Cluster.GetCluster(ctx)
	if err != nil {
		lgr.Error("Error getting name from cluster")
		return fmt.Errorf("error getting name from cluster")
	}
	ClusterName = cluster.Name

	svc, err := getServiceObj(ctx, infra.SubscriptionId, infra.ResourceGroup.GetName(), *ClusterName, infra.ServiceName)
	if err != nil {
		lgr.Error("Error getting service object")
		return fmt.Errorf("error getting service object")
	}
	Service = svc

	runTestFn := func(t test, ctx context.Context) *logger.LoggedError {
		lgr := logger.FromContext(ctx).With("test", t.GetName())
		ctx = logger.WithContext(ctx, lgr)
		lgr.Info("starting to run test")

		if err := t.Run(ctx); err != nil {
			return logger.Error(lgr, err)
		}

		lgr.Info("finished running test")
		return nil
	}

	publicZones := make([]string, len(infra.Zones))
	for i, zone := range infra.Zones {
		publicZones[i] = zone.GetId()
	}
	privateZones := make([]string, len(infra.PrivateZones))
	for i, zone := range infra.PrivateZones {
		privateZones[i] = zone.GetId()
	}

	//Loop to run ALL Tests
	lgr.Info("starting to run tests")

	var eg errgroup.Group
	for _, t := range allTests {
		func(t test) {
			eg.Go(func() error {
				if err := runTestFn(t, ctx); err != nil {
					return fmt.Errorf("running test: %w", err)
				}

				return nil
			})
		}(t)
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	lgr.Info("successfully finished running tests")
	return nil
}

func getServiceObj(ctx context.Context, subId, rg, clusterName, serviceName string) (*corev1.Service, error) {
	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("retrieving service object")
	defer lgr.Info("finished getting service")

	cmd := fmt.Sprintf("kubectl get service %s -n kube-system -o json", serviceName)
	resultProperties, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr(cmd),
	}, runCommandOpts{})

	if err != nil {
		return nil, fmt.Errorf("error getting service %s", serviceName)
	}
	responseLog := *resultProperties.Logs

	fmt.Println("annotation log: ", responseLog)
	svcObj := &corev1.Service{}
	err = json.Unmarshal([]byte(responseLog), svcObj)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling json for service: %s", err)
	}

	//success
	return svcObj, nil

}

func keys[T comparable, V any](m map[T]V) []T {
	ret := make([]T, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}

	return ret
}

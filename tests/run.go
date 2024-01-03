package tests

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"
)

// global exported vars used by tests
var (
	ClusterName   *string
	Ipv4Service   *corev1.Service
	Ipv6Service   *corev1.Service
	PublicZone    string
	PrivateZone   string
	ResourceGroup string
	SubId         string
)

func init() {
	log.SetLogger(logr.New(log.NullLogSink{})) // without this controller-runtime panics. We use it solely for the client so we can ignore logs

}

// Sets variables used by all tests
func SetObjectsForTesting(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("Setting objects for testing")

	//setting exported vars used by all tests
	cluster, err := infra.Cluster.GetCluster(ctx)
	if err != nil {
		lgr.Error("Error getting name from cluster")
		return fmt.Errorf("error getting name from cluster")
	}
	ClusterName = cluster.Name

	ipv4Svc, err := getServiceObj(ctx, infra.SubscriptionId, infra.ResourceGroup.GetName(), *ClusterName, infra.Ipv4ServiceName)
	if err != nil {
		lgr.Error("Error getting service object")
		return fmt.Errorf("error getting service object")
	}
	Ipv4Service = ipv4Svc

	ipv6Svc, err := getServiceObj(ctx, infra.SubscriptionId, infra.ResourceGroup.GetName(), *ClusterName, infra.Ipv6ServiceName)
	if err != nil {
		lgr.Error("Error getting service object")
		return fmt.Errorf("error getting service object")
	}
	Ipv6Service = ipv6Svc

	for _, zone := range infra.Zones {
		PublicZone = zone.GetName()
	}

	for _, zone := range infra.PrivateZones {
		PrivateZone = zone.GetName()
	}

	ResourceGroup = infra.ResourceGroup.GetName()
	SubId = infra.SubscriptionId

	return nil
}

func (allTests Ts) Run(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("Starting to run all tests in suite")

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

	//Loop to run ALL Tests
	lgr.Info("starting to run tests")

	for _, t := range allTests {
		func(t test) error {
			if err := runTestFn(t, ctx); err != nil {
				return fmt.Errorf("running test: %w", err)
			}
			return nil
		}(t)
	}
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

	svcObj := &corev1.Service{}
	err = json.Unmarshal([]byte(responseLog), svcObj)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling json for service: %s", err)
	}

	//success
	return svcObj, nil

}

package tests

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"
)

type IpFamily string

// enum for ip families
const (
	Ipv4  IpFamily = "IPv4"
	Ipv6  IpFamily = "IPv6"
	Cname IpFamily = "CNAME"
	Mx    IpFamily = "MX"
	Txt   IpFamily = "TXT"
)

var nonZeroExitCode = errors.New("non-zero exit code")

type runCommandOpts struct {
	// outputFile is the file to write the output of the command to. Useful for saving logs from a job or something similar
	// where there's lots of logs that are extremely important and shouldn't be muddled up in the rest of the logs.
	outputFile string
}

// Annotates Service with given key, value pair
func AnnotateService(ctx context.Context, subId, clusterName, rg, key, value, serviceName string) error {

	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to Annotate service")
	defer lgr.Info("finished annotating service")

	//TODO: namespace parameter
	cmd := fmt.Sprintf("kubectl annotate service --overwrite %s %s=%s -n kube-system", serviceName, key, value)

	if _, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr(cmd),
	}, runCommandOpts{}); err != nil {
		return fmt.Errorf("running kubectl apply: %w", err)
	}

	// //TODO: This check takes extra time, slows down tests. Do we actually need to check if annotation was saved? kubectl apply will fail if it doesn't anyways, right?
	// serviceObj, err := getServiceObj(ctx, subId, rg, clusterName, serviceName)
	// if err != nil {
	// 	return fmt.Errorf("error getting service object after annotating")
	// }

	// //check that annotation was saved
	// if serviceObj.Annotations[key] == value {
	// 	return nil
	// } else {
	// 	return fmt.Errorf("service annotation was not saved")
	// }

	return nil

}

// Removes all annotations except for last-applied-configuration which is needed by kubectl apply
// Called before test exits to clean up resources
func ClearAnnotations(ctx context.Context, subId, clusterName, rg, serviceName string) error {
	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to clear annotations")
	defer lgr.Info("finished removing all annotations on service")

	serviceObj, err := getServiceObj(ctx, subId, rg, clusterName, serviceName)
	if err != nil {
		return fmt.Errorf("error getting service object before clearing annotations")
	}

	annotations := serviceObj.Annotations
	for key := range annotations {
		if key != "kubectl.kubernetes.io/last-applied-configuration" {
			cmd := fmt.Sprintf("kubectl annotate service %s %s -n kube-system", serviceName, key+"-")

			if _, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
				Command: to.Ptr(cmd),
			}, runCommandOpts{}); err != nil {
				return fmt.Errorf("running kubectl apply: %w", err)
			}
		}
	}

	//TODO: namespace parameter
	serviceObj, err = getServiceObj(ctx, subId, rg, clusterName, serviceName)
	if err != nil {
		return fmt.Errorf("error getting service object after annotating")
	}

	//check that only last-applied-configuration annotation is left
	if len(serviceObj.Annotations) == 1 {
		lgr.Info("Cleared annotations successfully")
		return nil
	} else {
		return fmt.Errorf("service annotations not cleared")
	}

}

// TODO: param: add suport for PrivateProvider, which has a different ext dns deployment name. ADD PARAM instead of hardcoded "external-dns"
// Checks to see that external dns pod is running
func WaitForExternalDns(ctx context.Context, timeout time.Duration, subId, rg, clusterName string) error {
	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("Checking/ Waiting for external dns pod to run")
	defer lgr.Info("Done waiting for external dns pod")

	resultProperties, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr("kubectl get deploy external-dns -n kube-system -o json"), //TODO: provider as a param
	}, runCommandOpts{})

	if err != nil {
		return fmt.Errorf("unable to get pod for external-dns deployment")
	}

	//CHECK HERE
	responseLog := *resultProperties.Logs
	deploy := &appsv1.Deployment{}
	err = json.Unmarshal([]byte(responseLog), deploy)
	if err != nil {

		return fmt.Errorf("error with unmarshaling json")
	}

	var extDNSReady bool = true
	if deploy.Status.AvailableReplicas < 1 {
		var i int = 0
		for deploy.Status.AvailableReplicas < 1 {
			lgr.Info("======= ExternalDNS not available, checking again in %s seconds ====", timeout)
			time.Sleep(timeout)
			i++

			if i >= 5 {
				extDNSReady = false
			}
		}
	}

	if extDNSReady {
		lgr.Info("External Dns deployment is running and ready")
		return nil
	} else {
		return fmt.Errorf("external dns deployment is not running in pod, check logs")
	}

}

// Adds annotations needed specifically for private dns tests
// TODO: change annotate service to take a map of kep value pairs instead of adding one annotatoin at a time
func PrivateDnsAnnotations(ctx context.Context, subId, clusterName, rg, serviceName string) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("Adding annotations for private dns")

	err := AnnotateService(ctx, subId, clusterName, rg, "service.beta.kubernetes.io/azure-load-balancer-internal", "true", serviceName)
	if err != nil {
		lgr.Error("Error annotating service to create internal load balancer ", err)
		return fmt.Errorf("error: %s", err)
	}

	err = AnnotateService(ctx, subId, clusterName, rg, "external-dns.alpha.kubernetes.io/internal-hostname", "server-clusterip.example.com", serviceName)
	if err != nil {
		lgr.Error("Error annotating service for private dns", err)
		return fmt.Errorf("error: %s", err)
	}

	return nil

}

func RunCommand(ctx context.Context, subId, rg, clusterName string, request armcontainerservice.RunCommandRequest, opt runCommandOpts) (armcontainerservice.CommandResultProperties, error) {
	lgr := logger.FromContext(ctx)
	ctx = logger.WithContext(ctx, lgr)

	lgr.Info("starting to run command")
	defer lgr.Info("finished running command for testing")

	emptyResp := &armcontainerservice.CommandResultProperties{}
	cred, err := clients.GetAzCred()
	if err != nil {
		return *emptyResp, fmt.Errorf("getting az credentials: %w", err)
	}

	client, err := armcontainerservice.NewManagedClustersClient(subId, cred, nil)
	if err != nil {
		return *emptyResp, fmt.Errorf("creating aks client: %w", err)
	}

	poller, err := client.BeginRunCommand(ctx, rg, clusterName, request, nil)
	if err != nil {
		return *emptyResp, fmt.Errorf("starting run command: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return *emptyResp, fmt.Errorf("running command: %w", err)
	}

	logs := ""
	if result.Properties != nil && result.Properties.Logs != nil {
		logs = *result.Properties.Logs
	}

	if opt.outputFile != "" {

		outputFile, err := os.OpenFile(opt.outputFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {

			return *result.Properties, fmt.Errorf("creating output file %s: %w", opt.outputFile, err)
		}
		defer outputFile.Close()
		_, err = outputFile.WriteString(logs)
		if err != nil {
			return *result.Properties, fmt.Errorf("writing output file %s: %w", opt.outputFile, err)
		}
	} else {
		lgr.Info("using command logs, no output file specified")
	}

	if *result.Properties.ExitCode != 0 {
		lgr.Info(fmt.Sprintf("command failed with exit code %d", *result.Properties.ExitCode))
		return *result.Properties, nonZeroExitCode
	}

	return *result.Properties, nil
}

// TODO: function to delete A and AAAA records directly from each zone to make sure test command is generating new records each time
// Deletes record sets created in Azure DNS, needed for all tests to run properly
func DeleteRecordSet(ctx context.Context, clusterName, subId, rg, zoneName string, recordType armdns.RecordType) error {
	lgr := logger.FromContext(ctx)

	cred, err := clients.GetAzCred()
	if err != nil {
		lgr.Error("Error getting azure credentials")
		return err
	}

	clientFactory, err := armdns.NewClientFactory(subId, cred, nil)
	if err != nil {
		lgr.Error("failed to create client: %v", err)
		return err
	}
	_, err = clientFactory.NewRecordSetsClient().Delete(ctx, rg, zoneName, "@", recordType, &armdns.RecordSetsClientDeleteOptions{IfMatch: nil})
	if err != nil {
		lgr.Error("failed to delete record set: %v", err)
		return err
	}

	return nil
}

// TODO: function to delete A and AAAA records directly from each zone to make sure test command is generating new records each time
func DeletePrivateRecordSet(ctx context.Context, clusterName, subId, rg, zoneName string, recordType armprivatedns.RecordType) error {
	lgr := logger.FromContext(ctx)

	cred, err := clients.GetAzCred()
	if err != nil {
		lgr.Error("Error getting azure credentials")
		return err
	}

	clientFactory, err := armprivatedns.NewClientFactory(subId, cred, nil)
	if err != nil {
		lgr.Error("failed to create client: %v", err)
		return err
	}
	_, err = clientFactory.NewRecordSetsClient().Delete(ctx, rg, zoneName, recordType, "@", &armprivatedns.RecordSetsClientDeleteOptions{IfMatch: nil})
	if err != nil {
		lgr.Error("failed to delete record set: %v", err)
		return err
	}

	return nil
}

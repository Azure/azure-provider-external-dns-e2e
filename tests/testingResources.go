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

func AnnotateService(ctx context.Context, subId, clusterName, rg, serviceName string, annMap map[string]string) error {
	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to Annotate service")
	defer lgr.Info("finished annotating service")

	for key, value := range annMap {
		cmd := fmt.Sprintf("kubectl annotate service --overwrite %s %s=%s -n kube-system", serviceName, key, value)

		if _, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
			Command: to.Ptr(cmd),
		}, runCommandOpts{}); err != nil {
			return fmt.Errorf("running kubectl apply: %w", err)
		}
	}

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

// Checks to see that external dns pod is running
func WaitForExternalDns(ctx context.Context, numSeconds time.Duration, subId, rg, clusterName, provider string) error {
	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("Checking/ Waiting for external dns pod to run")
	defer lgr.Info("Done waiting for external dns pod")

	cmd := fmt.Sprintf("kubectl get deploy %s -n kube-system -o json", provider)
	resultProperties, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr(cmd),
	}, runCommandOpts{})

	if err != nil {
		return fmt.Errorf("unable to get pod for %s deployment", provider)
	}

	responseLog := *resultProperties.Logs
	deploy := &appsv1.Deployment{}
	err = json.Unmarshal([]byte(responseLog), deploy)
	if err != nil {

		return fmt.Errorf("error with unmarshaling json")
	}

	var extDNSReady bool = true
	timeout := time.Now().Add(numSeconds * time.Second)
	if deploy.Status.AvailableReplicas < 1 {
		var i int = 0
		for deploy.Status.AvailableReplicas < 1 {
			if time.Now().After(timeout) {
				return fmt.Errorf("external DNS deployment not ready after %s seconds", numSeconds)
			}
			lgr.Info("======= ExternalDNS not available, checking again in %s seconds ====", timeout)
			time.Sleep(2 * time.Second)
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
func PrivateDnsAnnotations(ctx context.Context, subId, clusterName, rg, serviceName string) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("Adding annotations for private dns")

	annotationMap := map[string]string{
		"external-dns.alpha.kubernetes.io/hostname":               PrivateZone,
		"service.beta.kubernetes.io/azure-load-balancer-internal": "true",
		"external-dns.alpha.kubernetes.io/internal-hostname":      "server-clusterip.example.com",
	}
	err := AnnotateService(ctx, subId, clusterName, rg, serviceName, annotationMap)
	if err != nil {
		lgr.Error("Error annotating service to create internal load balancer ", err)
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

// Deletes a record set in a public dns zone or private dns zone in Azure DNS
// Called after each test to allow subsequent test to run properly
func DeleteRecordSet(ctx context.Context, clusterName, subId, rg, zoneName string, recordType armdns.RecordType, privateRecordType armprivatedns.RecordType) error {
	lgr := logger.FromContext(ctx)

	lgr.Info("Starting to delete record set")
	defer lgr.Info("finished deleting record set")

	cred, err := clients.GetAzCred()
	if err != nil {
		lgr.Error("Error getting azure credentials")
		return err
	}

	if recordType != "" {
		clientFactory, err := armdns.NewClientFactory(subId, cred, nil)
		if err != nil {
			lgr.Error("failed to create client ", err)
			return err
		}
		_, err = clientFactory.NewRecordSetsClient().Delete(ctx, rg, zoneName, "@", recordType, &armdns.RecordSetsClientDeleteOptions{IfMatch: nil})
		if err != nil {
			lgr.Error("failed to delete record set in public dns zone ", err)
			return err
		}
	} else { //delete a private record set
		privateClientFactory, err := armprivatedns.NewClientFactory(subId, cred, nil)
		if err != nil {
			lgr.Error("failed to create client", err)
			return err
		}
		_, err = privateClientFactory.NewRecordSetsClient().Delete(ctx, rg, zoneName, privateRecordType, "@", &armprivatedns.RecordSetsClientDeleteOptions{IfMatch: nil})
		if err != nil {
			lgr.Error("failed to delete record set in private dns zone ", err)
			return err
		}

	}
	return nil
}

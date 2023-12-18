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

// Annotates Service and returns IP address on the load balancer
// adds annotation specifically under spec
func AnnotateService(ctx context.Context, subId, clusterName, rg, key, value, serviceName string) error {

	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to Annotate service")
	defer lgr.Info("finished annotating service")

	fmt.Println()
	fmt.Println("Annotation key: ", key)
	fmt.Println("Annotation value: ", value)
	fmt.Println()

	//TODO: namespace parameter
	cmd := fmt.Sprintf("kubectl annotate service --overwrite %s %s=%s -n kube-system", serviceName, key, value)

	if _, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr(cmd),
	}, runCommandOpts{}); err != nil {
		return fmt.Errorf("running kubectl apply: %w", err)
	}

	// //TODO: do we actually need to check if annotation was saved? kubectl apply will fail if it doesn't anyways, right?
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

// kubectl annotate service shopping-cart prometheus.io/scrape-
func ClearAnnotations(ctx context.Context, subId, clusterName, rg, serviceName string) error {
	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to clear annotations")
	defer lgr.Info("finished removing all annotations on service", serviceName)

	serviceObj, err := getServiceObj(ctx, subId, rg, clusterName, serviceName)
	if err != nil {
		return fmt.Errorf("error getting service object before clearing annotations")
	}

	fmt.Println()
	fmt.Println("Service Object before clearing annotations: ", serviceObj.Annotations)
	fmt.Println()

	annotations := serviceObj.Annotations
	for key := range annotations {
		if key != "kubectl.kubernetes.io/last-applied-configuration" {
			fmt.Println("removing key: ", key+"-")
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
	fmt.Println()
	fmt.Println("Service Object after clearing annotations: ", serviceObj.Annotations)
	fmt.Println()

	//check that annotation was saved
	if len(serviceObj.Annotations) == 1 {
		fmt.Println("Cleared annotations successfully ================ ")
		lgr.Info("Cleared annotations successfully")
		return nil
	} else {
		return fmt.Errorf("service annotations not cleared")
	}

}

// TODO: param: add suport for PrivateProvider, which has a different ext dns deployment name. ADD PARAM instead of hardcoded "external-dns"
func WaitForExternalDns(ctx context.Context, timeout time.Duration, subId, rg, clusterName string) error {
	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("Checking/ Waiting for external dns pod to run")
	defer lgr.Info("Done waiting for external dns pod")

	resultProperties, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr("kubectl get deploy external-dns -n kube-system -o json"),
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
			fmt.Printf("======= ExternalDNS not available, checking again in %s seconds ====", timeout)
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

// adds annotations needed specifically for private dns tests
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
	fmt.Println("IN RUN COMMAND for command for Tests +++++++++++++ command: ", *request.Command)

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
//func deleteARecordSet(rg, zoneName string, recordType) error{
//
// name: @.publiczone7d146e5d9158408d90a35c0ff27e21b8.com
//recordSetName = "@." + zoneName
// 			cmd := fmt.Sprintf("az network dns record-set a delete -g %s -n MyRecordSet -z www.mysite.com", rg, recordSetName, zoneName)

// 			if _, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
// 				Command: to.Ptr(cmd),
// 			}, runCommandOpts{}); err != nil {
// 				return fmt.Errorf("running kubectl apply: %w", err)
// 			}

//return fmt.Errorf("Error removing record set: ", recordType)
//}

//func deleteAAAARecordSet(rg, zoneName string, recordType) error{
//
//recordSetName = "@." + zoneName
// 			cmd := fmt.Sprintf("az network dns record-set aaaa delete -g %s -z %s -n %s", rg, zoneName, recordSetName)

// 			if _, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
// 				Command: to.Ptr(cmd),
// 			}, runCommandOpts{}); err != nil {
// 				return fmt.Errorf("running kubectl apply: %w", err)
// 			}

//return fmt.Errorf("Error removing record set: ", recordType)
//}

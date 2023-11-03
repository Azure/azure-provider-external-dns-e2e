package tests

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"
)

var cred azcore.TokenCredential
var nonZeroExitCode = errors.New("non-zero exit code")

type runCommandOpts struct {
	// outputFile is the file to write the output of the command to. Useful for saving logs from a job or something similar
	// where there's lots of logs that are extremely important and shouldn't be muddled up in the rest of the logs.
	outputFile string
}

// Annotates Service and returns IP address on the load balancer
func AnnotateService(ctx context.Context, subId, clusterName, rg, key, value, serviceName string) (string, error) {

	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to Annotate service")
	defer lgr.Info("finished annotating service")

	//TODO: namespace parameter
	cmd := fmt.Sprintf("kubectl annotate service --overwrite %s %s=%s -n kube-system", serviceName, key, value)
	fmt.Println("About to Run Command: ", cmd)
	if _, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr(cmd),
	}, runCommandOpts{}); err != nil {
		return "", fmt.Errorf("running kubectl apply: %w", err)
	}

	//check that annotation was saved, get IP address
	cmd = fmt.Sprintf("kubectl get service %s -n kube-system -o json", serviceName)
	resultProperties, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr(cmd),
	}, runCommandOpts{})

	if err != nil {
		return "", fmt.Errorf("error getting service %s", serviceName)
	}
	responseLog := *resultProperties.Logs

	fmt.Println("annotation log: ", responseLog)
	svcObj := &corev1.Service{}
	err = json.Unmarshal([]byte(responseLog), svcObj)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling json for service: %s", err)
	}

	//fmt.Println(svcObj.Annotations)

	if svcObj.Annotations[key] == value {
		return svcObj.Status.LoadBalancer.Ingress[0].IP, nil
	} else {
		return "", fmt.Errorf("service annotation was not saved")
	}

}

// TODO: param: add suport for PrivateProvider, which has a different ext dns deployment name. ADD PARAM instead of hardcoded "external-dns"
// TODO: Create logger for all fmt.Printlns in this fn
func WaitForExternalDns(ctx context.Context, timeout time.Duration, subId, rg, clusterName string) error {
	//fmt.Println("In WaitForExternalDns() function ----------------")

	resultProperties, err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr("kubectl get deploy external-dns -n kube-system -o json"),
	}, runCommandOpts{outputFile: "testLogs.txt"})

	if err != nil {
		fmt.Println("Error is not nil, not able to execute command for deployment")
		return fmt.Errorf("unable to get pod for external-dns deployment")
	}

	//CHECK HERE
	responseLog := *resultProperties.Logs
	deploy := &appsv1.Deployment{}
	err = json.Unmarshal([]byte(responseLog), deploy)
	if err != nil {
		fmt.Println("error unmarshaling ===========")
		return fmt.Errorf("error with unmarshaling json")
	}

	fmt.Println()
	fmt.Println("=======================================")
	fmt.Println("About to check available replicas")

	var extDNSReady bool = true
	if deploy.Status.AvailableReplicas < 1 {
		var i int = 0
		for deploy.Status.AvailableReplicas < 1 {
			fmt.Printf("======= ExternalDNS not available, checking again in %s seconds ====", timeout)
			time.Sleep(timeout)
			i++

			if i >= 5 {
				fmt.Println("Done waiting for External DNS, pod is NOT running")
				extDNSReady = false
			}
		}
	}

	if extDNSReady {
		fmt.Println("ExternalDNS Deployment is ready")
		fmt.Println("=======================================")
		fmt.Println()
		return nil
	} else {
		return fmt.Errorf("external dns deployment is not running in pod, check logs")
	}

}

func RunCommand(ctx context.Context, subId, rg, clusterName string, request armcontainerservice.RunCommandRequest, opt runCommandOpts) (armcontainerservice.CommandResultProperties, error) {
	fmt.Println("IN RUN COMMAND for command for Tests +++++++++++++ command: ", *request.Command)

	lgr := logger.FromContext(ctx)
	ctx = logger.WithContext(ctx, lgr)

	lgr.Info("starting to run command")
	defer lgr.Info("finished running command for testing")

	emptyResp := &armcontainerservice.CommandResultProperties{}
	//fmt.Println("#1 Before getting az creds")
	cred, err := getAzCred()
	if err != nil {
		return *emptyResp, fmt.Errorf("getting az credentials: %w", err)
	}

	//fmt.Println("#2 Before creating managed cluster client")
	client, err := armcontainerservice.NewManagedClustersClient(subId, cred, nil)
	if err != nil {
		return *emptyResp, fmt.Errorf("creating aks client: %w", err)
	}

	//fmt.Println("#3 Before BeginRunCommand() call")
	poller, err := client.BeginRunCommand(ctx, rg, clusterName, request, nil)
	if err != nil {
		return *emptyResp, fmt.Errorf("starting run command: %w", err)
	}

	//fmt.Println("#4 Before Poller PollUnitlDone() call")
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return *emptyResp, fmt.Errorf("running command: %w", err)
	}

	//fmt.Println("#5 Before Checking Logs")
	logs := ""
	if result.Properties != nil && result.Properties.Logs != nil {
		logs = *result.Properties.Logs
		//fmt.Println("logs from run command in testing: ", logs)
	}

	//fmt.Println("#5 Before output file code")
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

	//fmt.Println("#6 Before checking exit code")
	if *result.Properties.ExitCode != 0 {
		lgr.Info(fmt.Sprintf("command failed with exit code %d", *result.Properties.ExitCode))
		return *result.Properties, nonZeroExitCode
	}

	//fmt.Println("returning logs with no error >>>>>>>>>>>>>>>>>>>>>")
	return *result.Properties, nil
}

func getAzCred() (azcore.TokenCredential, error) {
	if cred != nil {
		return cred, nil
	}

	// this is CLI instead of DefaultCredential to ensure we are using the same credential as the CLI
	// and authed through the cli. We use the az cli directly when pushing an image to ACR for now.
	c, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("getting az cli credential: %w", err)
	}

	cred = c
	return cred, nil
}

package tests

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"
)

var cred azcore.TokenCredential
var nonZeroExitCode = errors.New("non-zero exit code")

type runCommandOpts struct {
	// outputFile is the file to write the output of the command to. Useful for saving logs from a job or something similar
	// where there's lots of logs that are extremely important and shouldn't be muddled up in the rest of the logs.
	outputFile string
}

type aks struct {
	name, subscriptionId, resourceGroup string
	id                                  string
	dnsServiceIp                        string
	location                            string
	principalId                         string
	clientId                            string
	options                             map[string]struct{}
}

// addl param: objs []client.Object
func AnnotateService(ctx context.Context, subId, clusterName, rg, key, value, serviceName string) error {
	fmt.Println("In AnnotateService function --------------------")

	lgr := logger.FromContext(ctx).With("name", clusterName, "resourceGroup", rg)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("starting to Annotate service")
	defer lgr.Info("finished annotating service")

	//TODO: namespace parameter
	cmd := fmt.Sprintf("kubectl annotate service --overwrite %s %s=%s -n kube-system", serviceName, key, value)
	fmt.Println("About to Run Command: ", cmd)
	if err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr(cmd),
	}, runCommandOpts{}); err != nil {
		return fmt.Errorf("running kubectl apply: %w", err)
	}

	return nil
}

// param: add suuport for PrivateProvider, which has a different ext dns deployment name. ADD PARAM instead of hardcoded "external-dns"
func WaitForExternalDns(ctx context.Context, timeout time.Duration, subId, rg, clusterName string) error {
	fmt.Println("In wait for external dns function ----------------")
	//kubectl get pods --selector=app=external-dns -A

	if err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
		Command: to.Ptr("kubectl get pods --selector=app=external-dns -A"),
	}, runCommandOpts{outputFile: "testLogs.txt"}); err != nil {
		return fmt.Errorf("unable to get pod for external-dns deployment")
	}

	// if err := RunCommand(ctx, subId, rg, clusterName, armcontainerservice.RunCommandRequest{
	// 	Command: to.Ptr(fmt.Sprintf("kubectl wait --for=condition=Ready pod/%s -n %s", "external-dns", "kube-system")),
	// }, runCommandOpts{}); err != nil {
	// 	return fmt.Errorf("waiting for pod/%s to be stable: %w", "external-dns", err)
	// }
	fmt.Println("Returning success from WaitForExternalDns() ----------------------")
	return nil
}

func RunCommand(ctx context.Context, subId, rg, clusterName string, request armcontainerservice.RunCommandRequest, opt runCommandOpts) error {
	fmt.Println("IN RUN COMMAND for command for Tests +++++++++++++: ", request.Command)

	lgr := logger.FromContext(ctx)
	ctx = logger.WithContext(ctx, lgr)

	lgr.Info("starting to run command")
	defer lgr.Info("finished running command for testing")

	cred, err := getAzCred()
	if err != nil {
		return fmt.Errorf("getting az credentials: %w", err)
	}

	client, err := armcontainerservice.NewManagedClustersClient(subId, cred, nil)
	if err != nil {
		return fmt.Errorf("creating aks client: %w", err)
	}

	poller, err := client.BeginRunCommand(ctx, rg, clusterName, request, nil)
	if err != nil {
		return fmt.Errorf("starting run command: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("running command: %w", err)
	}

	fmt.Println("------------ result: ", result)
	fmt.Println("Got here #5 =========================")

	logs := ""
	if result.Properties != nil && result.Properties.Logs != nil {
		logs = *result.Properties.Logs
		fmt.Println("logs from run command in testing: ", logs)
	}
	fmt.Println("Got here #6 =========================")

	if opt.outputFile != "" {

		outputFile, err := os.OpenFile(opt.outputFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {

			return fmt.Errorf("creating output file %s: %w", opt.outputFile, err)
		}
		defer outputFile.Close()
		_, err = outputFile.WriteString(logs)
		if err != nil {
			return fmt.Errorf("writing output file %s: %w", opt.outputFile, err)
		}
	} else {
		lgr.Info("command output: " + logs)
	}

	fmt.Println("Got here #7 =========================")

	if *result.Properties.ExitCode != 0 {
		lgr.Info(fmt.Sprintf("command failed with exit code %d", *result.Properties.ExitCode))
		return nonZeroExitCode
	}

	return nil
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

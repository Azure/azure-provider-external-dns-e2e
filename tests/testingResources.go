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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
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

var cred azcore.TokenCredential
var nonZeroExitCode = errors.New("non-zero exit code")
var basicNs = make(map[string]*corev1.Namespace)

type runCommandOpts struct {
	// outputFile is the file to write the output of the command to. Useful for saving logs from a job or something similar
	// where there's lots of logs that are extremely important and shouldn't be muddled up in the rest of the logs.
	outputFile string
}

// adds ip record type to Service spec.ipFamilies and spec.ipFamilyPolicy.. used mainly for ipv6 dual stack clusters
// but using it just for single stack testing to test ip families individually
// zones passed in must be EITHER be a public or private zone based on what the test requires
func AddIPFamilySpec(ctx context.Context, infra infra.Provisioned, recordType IpFamily, ipFamilyPolicy corev1.IPFamilyPolicy, usePublicZone bool) error {

	lgr := logger.FromContext(ctx).With("name", ClusterName, "resourceGroup", ResourceGroup)
	ctx = logger.WithContext(ctx, lgr)
	lgr.Info("Starting to update IP spec on Service")
	defer lgr.Info("Finished updating IP spec")

	ipFamilies := make([]corev1.IPFamily, 1)
	protocol := recordType.convertValue()
	ipFamilies[0] = protocol

	Service.Spec.IPFamilyPolicy = &ipFamilyPolicy
	Service.Spec.IPFamilies = ipFamilies

	//Debug statements ---
	fmt.Println("==========================================")
	fmt.Println()
	fmt.Println("In memory ip family policy: ", Service.Spec.IPFamilyPolicy)
	fmt.Println("In memory Service: ", Service.Spec.IPFamilies)
	fmt.Println("==========================================")
	fmt.Println()

	//get kubeconfig
	cred, err := getAzCred()
	if err != nil {
		return fmt.Errorf("getting az credentials: %w", err)
	}

	clientFactory, err := armcontainerservice.NewClientFactory(SubscriptionId, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	res, err := clientFactory.NewManagedClustersClient().ListClusterAdminCredentials(ctx, ResourceGroup, *ClusterName, nil)
	if err != nil {
		return fmt.Errorf("Unable to create managed clusters client")
	}
	kubeconfig := res.Kubeconfigs[0]

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig.Value)

	if err != nil {
		return fmt.Errorf("Unable to create rest config from kubeconfig")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Unable to clientset from rest config")
	}

	serviceInterface := clientset.CoreV1().Services("kube-system") //TODO: pass in namespace
	updatedService, err := serviceInterface.Update(ctx, Service, v1.UpdateOptions{})
	fmt.Println()
	fmt.Println("=======================================")
	fmt.Println("ip families: ", updatedService.Spec.IPFamilies)
	fmt.Println("ip family policy:", updatedService.Spec.IPFamilyPolicy)
	fmt.Println("=======================================")
	fmt.Println()

	//res, err := clientFactory.NewManagedClustersClient().GetAccessProfile(ctx, ResourceGroup, *ClusterName, "clusterUser", nil)
	if err != nil {
		return fmt.Errorf("failed to finish the request: %v", err)
	}

	return nil

}

func (ip IpFamily) convertValue() corev1.IPFamily {

	switch ip {

	case Ipv4:
		return corev1.IPv4Protocol
	case Ipv6:
		return corev1.IPv6Protocol
	case Cname:
		//TODO
	case Mx:
		//TODO
	case Txt:
		//TODO
	}

	fmt.Println("returning default ipv4 protocol")
	//default is an ipv4 address, change this later?
	return corev1.IPv4Protocol

}

// Annotates Service and returns IP address on the load balancer
// adds annotation specifically under spec
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

	serviceObj, err := getServiceObj(ctx, subId, rg, clusterName, serviceName)
	if err != nil {
		return fmt.Errorf("error getting service object after annotating")
	}

	//check that annotation was saved, get IP address
	if serviceObj.Annotations[key] == value {
		return nil
	} else {
		return fmt.Errorf("service annotation was not saved")
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
	}, runCommandOpts{outputFile: "testLogs.txt"})

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

func upsert(ctx context.Context, c client.Client, obj client.Object) error {
	copy := obj.DeepCopyObject().(client.Object)
	lgr := logger.FromContext(ctx).With("object", copy.GetName(), "namespace", copy.GetNamespace())
	lgr.Info("upserting object")

	// create or update the object
	lgr.Info("attempting to create object")
	err := c.Create(ctx, copy)
	if err == nil {
		obj.SetName(copy.GetName()) // supports objects that want to use generate name
		lgr.Info("object created")
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating object: %w", err)
	}

	lgr.Info("object already exists, attempting to update")
	if err := c.Update(ctx, copy); err != nil {
		return fmt.Errorf("updating object: %w", err)
	}

	lgr.Info("object updated")
	return nil
}

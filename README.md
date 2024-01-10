# ExternalDNS E2E tests

## What is ExternalDNS?
ExternalDNS allows kubernetes services to be discorvered by public DNS servers. We can configure a service so it has direct access to your own DNS provider, such as Azure DNS 

ExternalDNS works by scaning the service's current config on specific tags which will tell the ExternalDNS service that it should update Azure DNS with the provided URI in the tag.

The test plan for the tests, along with more detailed steps on provisioning and testing on infrastructure can be found [here](https://msazure.visualstudio.com/CloudNativeCompute/_wiki/wikis/personalplayground?wikiVersion=GBmain&pagePath=/Manasa%20Chinta/External%252Ddns%20e2e%20testing&pageId=598453&_a=edit).
The source code for external-dns can be found [here](https://github.com/kubernetes-sigs/external-dns).
Tutorial for the Azure Provider for externalDNS can be found [here](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/azure.md).
## Steps for running tests locally:
(started by calling infra command under cmd/ folder)

<b>Run e2e locally with the following steps: </b>
- Ensure you've copied the .env.example file to .env and filled in the values. You can replace the `INFRA_NAMES` value in the .env file with the name of any infrastructure defined in infra/infras.go to test different scenarios. `"basic cluster"` and `"private cluster."` 
- Run `make e2e`. This runs the infra command then the test command
   - The resources provisoned are written to a .json file (infra_config.json if running locally, infra.json is the default for workflows).
   - You can tell if a test has passed by searching for "passed" in the logs printed to the terminal.
   - Current tests create A and AAAA records in public and private dns zones
- To run tests on a different version of external-dns, modify the version in the deployment spec in external_dns.go -> newExternalDNSDeployment() function:
    ![alt text](/images/extdns-version.jpg "external dns version modification") 
***
<b>Note:</b>
- Infrastructures are defined in /infra/infras.go. Add any new AKS cluster configurations here.
- Tests are defined in /suites. Add any new tests here. If multiple suites are needed, they should be added to/suites/all.go so that they are run.
***

## Running tests through github workflows

Github workflows are set up to run and require passing E2E tests on every PR. 
The e2ev2-provision-test.yaml Github Workflow will provision and run tests on both infras.

A successful build will show the e2ev2, status, and validation-tests jobs completed on the left
![alt text](/images/workflow-success.jpg "external dns version modification") 
If a step fails you have a few options for debugging.

- Read the logs printed under the provision or test steps.
- Connect to the Kubernetes cluster and dig around manually. The logs should include information on the cluster name, resource group, and subscription that you can use to connect to the cluster.



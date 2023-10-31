package suites

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	corev1 "k8s.io/api/core/v1"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	"github.com/Azure/azure-provider-external-dns-e2e/tests"
)

var serviceObj *corev1.Service

func basicSuite(in infra.Provisioned) []test {

	log.Printf("In basic suite >>>>>>>>>>>>>>>>>>>>>>")
	return []test{

		{
			name: "public + A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
			run: func(ctx context.Context) error {

				if err := ARecordTest(ctx, in); //func(service *corev1.Service) error {

				err != nil {
					return err
				}

				return nil
			},
		},
	}
}

// modifier is a function that can be used to modify the ingress and service
//type modifier func(service *corev1.Service) error

var ARecordTest = func(ctx context.Context, infra infra.Provisioned) error {
	fmt.Printf("%+v\n", infra)

	fmt.Println("In basic record test --------------")
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	publicZone := infra.Zones[0]

	cluster, err := infra.Cluster.GetCluster(ctx)
	if err != nil {
		lgr.Error("Error getting name from cluster")
	}
	clusterName := cluster.Name

	//values:
	fmt.Println("Infra subId: ", infra.SubscriptionId)
	fmt.Println("Infra cluster name: ", *clusterName)
	fmt.Println("Infra rg: ", infra.ResourceGroup.GetName())
	fmt.Println("Infra zone name: ", publicZone.GetName())
	fmt.Println("Infra Service name: ", infra.Service)

	serviceName := infra.Service
	err = tests.AnnotateService(ctx, infra.SubscriptionId, *clusterName, infra.ResourceGroup.GetName(), "external-dns.alpha.kubernetes.io/hostname", publicZone.GetName(), serviceName)
	if err != nil {
		//fmt.Println(err.(*errors.Error).ErrorStack())
		lgr.Error("Error annotating service", err)
	}

	err = validateRecord(ctx, armdns.RecordTypeA, infra.ResourceGroup.GetName(), infra.SubscriptionId, *clusterName, publicZone.GetName(), 2, serviceName)
	if err != nil {
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeA)
	} else {
		lgr.Info("finished successfully")
	}

	return nil
}

// Checks to see whether record is created in Azure DNS
// time out value param
func validateRecord(ctx context.Context, recordType armdns.RecordType, rg, subscriptionId, clusterName, dnsZoneName string, timeout time.Duration, serviceName string) error {
	fmt.Println()
	fmt.Println("#0 In validateRecord() function ---------------------")
	//TODO: timeout loop to wait on external dns
	//for loop that tests to see if
	//checks every timeout interval if external dns is done, once it's up and running, it breaks

	lgr := logger.FromContext(ctx)
	lgr.Info("Checking that Record was created in Azure DNS")

	err := tests.WaitForExternalDns(ctx, timeout, subscriptionId, rg, clusterName)
	if err != nil {
		return fmt.Errorf("error waiting for ExternalDNS to start running %w", err)
	}

	cred, err := clients.GetAzCred()
	if err != nil {
		return fmt.Errorf("getting az credentials: %w", err)
	}
	// ctx = context.Background()

	fmt.Println("#1 Creating ClientFactory function ---------------------")
	clientFactory, err := armdns.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
		return fmt.Errorf("failed to create armdns.ClientFactory")
	}
	fmt.Println("#2 Creating pager ---------------------")
	pager := clientFactory.NewRecordSetsClient().NewListByTypePager(rg, dnsZoneName, recordType, &armdns.RecordSetsClientListByTypeOptions{Top: nil,
		Recordsetnamesuffix: nil,
	})

	for pager.More() {
		fmt.Println("#3 In pager ----------------")
		page, err := pager.NextPage(ctx)

		if err != nil {
			log.Fatalf("failed to advance page: %v", err)
			return fmt.Errorf("failed to advance page for record sets")
		}
		for _, v := range page.Value {
			fmt.Println("In Loop!  dns record created ======================= :)")
			//fmt.Println("Record type: ", *(v.Type))
			fmt.Println("#4 =========== Ip address: ", *(v.Properties.ARecords[0]))
			fmt.Println("#5 =========== Zone name: ", *(v.Properties.Fqdn))

			//TODO: grab a dns record whose dns zone name matches the service dns zone name. Then check to see if the ip address matches the
			//ip address on the load balancer

			//checking one Value at a time?
			// if *(v.Type) != "Microsoft.Network/dnsZones/A" {
			// 	return fmt.Errorf("A record not created in Azure DNS, test failed")
			// }

			//if (v.Properties.ARecords[0][IPv4Address]) -- TODO: check IP addr. Figure out what else we should check

		}

	}

	// If the HTTP response code is 200 as defined in example definition, your page structure would look as follows. Please pay attention that all the values in the output are fake values for just demo purposes.
	// page.RecordSetListResult = armdns.RecordSetListResult{
	// 	Value: []*armdns.RecordSet{
	// 		{
	// 			Name: to.Ptr("record1"),
	// 			Type: to.Ptr("Microsoft.Network/dnsZones/A"),
	// 			Etag: to.Ptr("00000000-0000-0000-0000-000000000000"),
	// 			ID: to.Ptr("/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/dnsZones/zone1/A/record1"),
	// 			Properties: &armdns.RecordSetProperties{
	// 				ARecords: []*armdns.ARecord{
	// 					{
	// 						IPv4Address: to.Ptr("127.0.0.1"),
	// 				}},
	// 				TTL: to.Ptr[int64](3600),
	// 				Fqdn: to.Ptr("record1.zone1"),
	// 				Metadata: map[string]*string{
	// 					"key1": to.Ptr("value1"),
	// 				},
	// 			},
	// 	}},
	// }

	return nil
}

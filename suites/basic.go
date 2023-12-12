package suites

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	"github.com/Azure/azure-provider-external-dns-e2e/tests"
)

func basicSuite(in infra.Provisioned) []test {

	log.Printf("In basic suite >>>>>>>>>>>>>>>>>>>>>>")
	return []test{
		{
			name: "public cluster + public DNS +  A Record",
			run: func(ctx context.Context) error {
				fmt.Println("**********************************")
				fmt.Println("Test public DNS + A record")
				fmt.Println("**********************************")
				if err := ARecordTest(ctx, in, true); err != nil {
					fmt.Println("BAD A public ======================= ")
					return err
				}

				return nil
			},
		},
		{
			name: "public cluster + public DNS +  Quad A Record",
			run: func(ctx context.Context) error {

				fmt.Println("**********************************")
				fmt.Println("Test public DNS + AAAA record")
				fmt.Println("**********************************")

				if err := AAAARecordTest(ctx, in, true); err != nil {
					fmt.Println("BAD AAAA public ======================= ")

					return err
				}
				return nil
			},
		},
		{
			name: "removing annotations",
			run: func(ctx context.Context) error {

				err := tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
				if err != nil {
					return err
				}
				return nil
			},
		},
	}
}

var AAAARecordTest = func(ctx context.Context, infra infra.Provisioned, usePublicZone bool) error {

	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId
	publicZone := infra.Zones[0].GetName()
	ipv6ServiceName := infra.Ipv6ServiceName

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", publicZone, infra.Ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	err = tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", publicZone, ipv6ServiceName)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	//Validating records
	err = validateRecord(ctx, armdns.RecordTypeAAAA, resourceGroup, subId, *tests.ClusterName, publicZone, 50, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeAAAA)
	} else {
		lgr.Info("finished successfully")
	}

	return nil

}

var ARecordTest = func(ctx context.Context, infra infra.Provisioned, usePublicZone bool) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting A record test")

	//Currently only provisioning one public and one private zone, no test in this suite tests with more than one of each
	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId
	publicZone := infra.Zones[0].GetName()
	ipv4ServiceName := infra.Ipv4ServiceName

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", publicZone, ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service with zone name", err)
		return fmt.Errorf("error: %s", err)
	}

	err = validateRecord(ctx, armdns.RecordTypeA, resourceGroup, subId, *tests.ClusterName, publicZone, 20, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeA)
	} else {
		lgr.Info("finished successfully")
	}

	fmt.Println("End of A Record Test ============ ")
	return nil
}

// Checks to see whether record is created in Azure DNS
func validateRecord(ctx context.Context, recordType armdns.RecordType, rg, subscriptionId, clusterName, serviceDnsZoneName string, numSeconds time.Duration, svcIp string) error {

	lgr := logger.FromContext(ctx)
	lgr.Info("Checking that Record was created in Azure DNS")

	err := tests.WaitForExternalDns(ctx, numSeconds, subscriptionId, rg, clusterName)
	if err != nil {
		return fmt.Errorf("error waiting for ExternalDNS to start running %w", err)
	}

	cred, err := clients.GetAzCred()
	if err != nil {
		return fmt.Errorf("getting az credentials: %w", err)
	}

	clientFactory, err := armdns.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
		return fmt.Errorf("failed to create armdns.ClientFactory")
	}

	pager := clientFactory.NewRecordSetsClient().NewListByTypePager(rg, serviceDnsZoneName, recordType, &armdns.RecordSetsClientListByTypeOptions{Top: nil,
		Recordsetnamesuffix: nil,
	})

	timeout := time.Now().Add(numSeconds * time.Second)

	var pageValue []*armdns.RecordSet
	for {
		//fmt.Println("In loop for checking record ========= ")
		if time.Now().After(timeout) {
			return fmt.Errorf("record not created within %s seconds", numSeconds)
		}

		if pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				log.Fatalf("failed to advance page: %v", err)
				return fmt.Errorf("failed to advance page for record sets")
			}
			if len(page.Value) > 0 {
				pageValue = page.Value
				break
			}

		}
		//fmt.Println("In loop for checking record ========= ")
	}
	var ipAddr string
	for _, v := range pageValue {
		fmt.Println("In pageValue ==== record created ====== :))))")
		currZoneName := strings.Trim(*(v.Properties.Fqdn), ".") //removing trailing '.'

		if recordType == armdns.RecordTypeA {
			ipAddr = *(v.Properties.ARecords[0].IPv4Address)
		} else if recordType == armdns.RecordTypeAAAA {
			ipAddr = *(v.Properties.AaaaRecords[0].IPv6Address)
		} else {
			return fmt.Errorf("unable to match record type")
		}

		fmt.Println("#4 =========== Ip address: ", ipAddr)
		fmt.Println("#5 =========== Zone name: ", currZoneName)

		if currZoneName == serviceDnsZoneName && ipAddr == svcIp {
			fmt.Println()
			fmt.Println(" ======================== Record matched!!! ======================= ")

			return nil
		}

	}

	fmt.Println("Service ip: ", svcIp)
	fmt.Println("Service zone name:", serviceDnsZoneName)
	// for pager.More() {
	// 	fmt.Println("#3 In pager ----------------")
	// 	page, err := pager.NextPage(ctx)

	// 	if err != nil {
	// 		log.Fatalf("failed to advance page: %v", err)
	// 		return fmt.Errorf("failed to advance page for record sets")
	// 	}
	// 	fmt.Println("Page.value length: ", len(page.Value))
	// 	for _, v := range page.Value {
	// 		fmt.Println()
	// 		fmt.Println("In Loop!  dns record created ======================= :)")
	// 		fmt.Println()

	// 		currZoneName := strings.Trim(*(v.Properties.Fqdn), ".") //removing trailing '.'

	// 		if recordType == armdns.RecordTypeA {
	// 			ipAddr = *(v.Properties.ARecords[0].IPv4Address)
	// 		} else if recordType == armdns.RecordTypeAAAA {
	// 			ipAddr = *(v.Properties.AaaaRecords[0].IPv6Address)
	// 		} else {
	// 			return fmt.Errorf("unable to match record type")
	// 		}

	// 		fmt.Println("#4 =========== Ip address: ", ipAddr)
	// 		fmt.Println("#5 =========== Zone name: ", currZoneName)

	// 		if currZoneName == serviceDnsZoneName && ipAddr == svcIp {
	// 			fmt.Println()
	// 			fmt.Println(" ======================== Record matched!!! ==================== ")

	// 			return nil
	// 		}

	// 	}

	// }
	// //test failed
	return fmt.Errorf("record not created %s", recordType)

}

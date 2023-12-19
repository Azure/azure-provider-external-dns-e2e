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
			name: "public cluster + public DNS +  A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
			run: func(ctx context.Context) error {
				fmt.Println("**********************************")
				fmt.Println("Test public DNS + A record")
				fmt.Println("**********************************")
				lgr := logger.FromContext(ctx)
				//err := tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
				// if err != nil {
				// 	lgr.Error("Error clearing annotations for service (ipv4)", err)
				// 	return err
				// }
				if err := ARecordTest(ctx, in); err != nil {
					fmt.Println()
					fmt.Println("######################### BAD A public ######################### ")
					fmt.Println()
					//tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
					return err
				}
				lgr.Info("finished successfully, clearing annotions for service (ipv4)")
				fmt.Println("SUCCESS === Before clearing annotations for service (ipv4)")
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)

				return nil
			},
		},
		{
			name: "public cluster + public DNS +  Quad A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
			run: func(ctx context.Context) error {

				fmt.Println("**********************************")
				fmt.Println("Test public DNS + AAAA record")
				fmt.Println("**********************************")
				//lgr := logger.FromContext(ctx)
				// err := tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
				// if err != nil {
				// 	lgr.Error("Error clearing annotations for service (ipv4)", err)
				// 	return err
				// }

				// err = tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv6Service.Name)
				// if err != nil {
				// 	lgr.Error("Error clearing annotations for service (ipv6)", err)
				// 	return err
				// }
				if err := AAAARecordTest(ctx, in); err != nil {
					fmt.Println()
					fmt.Println("######################### BAD AAAA public ######################### ")
					fmt.Println()
					//tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
					//tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv6Service.Name)
					return err
				}
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv6Service.Name)
				return nil
			},
		},
	}
}

var ARecordTest = func(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting public dns + A record test")

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

	//checking to see if A record was created in Azure DNS
	time.Sleep(20 * time.Second)
	err = validateRecord(ctx, armdns.RecordTypeA, resourceGroup, subId, *tests.ClusterName, publicZone, 20, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeA)
	} else {
		lgr.Info("finished successfully")
	}

	//test passed, deleting record
	err = tests.DeleteRecordSet(ctx, *tests.ClusterName, subId, resourceGroup, publicZone, armdns.RecordTypeA)
	if err != nil {
		lgr.Error("Error deleting A record set")
		return fmt.Errorf("error deleting A record set")
	}
	return nil
}

var AAAARecordTest = func(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting public dns + AAAA test")

	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId
	publicZone := infra.Zones[0].GetName()
	ipv6ServiceName := infra.Ipv6ServiceName
	ipv4ServiceName := infra.Ipv4ServiceName

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", publicZone, ipv6ServiceName)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	fmt.Println()
	fmt.Println("After annotating ipv6 service ====================== Going into second")
	err = tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", publicZone, ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	fmt.Println()
	fmt.Println("After annotating second (ipv4) service ====================== ")

	fmt.Println("before sleeping in public AAAA test =========== ")
	time.Sleep(20 * time.Second)
	fmt.Println("After sleeping, waiting for record ==== going into Validate Record ")

	//Checking Azure DNS for AAAA record
	err = validateRecord(ctx, armdns.RecordTypeAAAA, resourceGroup, subId, *tests.ClusterName, publicZone, 50, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)
	fmt.Println("After validateRecord returns ======================= ")

	if err != nil {
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeAAAA)
	} else {
		lgr.Info("finished successfully")
	}

	//TODO: add delete record set for A and AAAA
	err = tests.DeleteRecordSet(ctx, *tests.ClusterName, subId, resourceGroup, publicZone, armdns.RecordTypeAAAA)
	if err != nil {
		lgr.Error("Error deleting AAAA record set")
		return fmt.Errorf("error deleting AAAA record set")
	}
	return nil

}

// Checks to see whether record is created in Azure DNS
func validateRecord(ctx context.Context, recordType armdns.RecordType, rg, subscriptionId, clusterName, serviceDnsZoneName string, numSeconds time.Duration, svcIp string) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("Checking that Record was created in Azure DNS")
	fmt.Println("Checking that Record was created in Azure DNS ============== Going into Wait for external dns")

	err := tests.WaitForExternalDns(ctx, numSeconds, subscriptionId, rg, clusterName)
	fmt.Println("Done waiting for external dns pod running ==================== ")
	if err != nil {
		fmt.Println("External dns pod not running!!! ===================== ")
		return fmt.Errorf("error waiting for ExternalDNS to start running %w", err)
	}

	fmt.Println("Before getting azure credentials ================ ")
	cred, err := clients.GetAzCred()
	if err != nil {
		return fmt.Errorf("getting az credentials: %w", err)
	}

	fmt.Println("Got azure credentials ==================== :)")

	clientFactory, err := armdns.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
		return fmt.Errorf("failed to create armdns.ClientFactory")
	}
	fmt.Println("Created ClientFactory ==================== :)")

	timeout := time.Now().Add(numSeconds * time.Second)

	var pageValue []*armdns.RecordSet
	fmt.Println("Before entering for loop ==================== ")
	for {
		fmt.Println("entered for loop ===================== ")
		if time.Now().After(timeout) {
			fmt.Println("Returning error here from Validate Record FAILED=========================== ")
			return fmt.Errorf("record not created within %s seconds", numSeconds)
		}
		fmt.Println("After checking if time is over ==========, creaing pager ")
		pager := clientFactory.NewRecordSetsClient().NewListByTypePager(rg, serviceDnsZoneName, recordType, &armdns.RecordSetsClientListByTypeOptions{Top: nil,
			Recordsetnamesuffix: nil,
		})
		fmt.Println("After creating pager ==================== ")
		if pager.More() {
			fmt.Println("In pager.More ====== ")
			page, err := pager.NextPage(ctx)
			if err != nil {
				log.Fatalf("failed to advance page: %v", err)
				return fmt.Errorf("failed to advance page for record sets")
			}
			fmt.Println("After page.NextPage(), no error ======== ")
			if len(page.Value) > 0 {
				pageValue = page.Value
				break
			}

		}
		time.Sleep(2 * time.Second) //TODO: add interval to check

	}
	fmt.Println("After for loop ==================== :)")

	fmt.Println("Before checking for record. pageValue: ", pageValue)
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
	//test failed
	fmt.Println("returning record not created === FAILURE ==== ")
	return fmt.Errorf("record not created %s", recordType)

}

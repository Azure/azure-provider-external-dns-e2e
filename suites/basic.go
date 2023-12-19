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
	return []test{
		{
			name: "public cluster + public DNS +  A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
			run: func(ctx context.Context) error {
				lgr := logger.FromContext(ctx)

				if err := ARecordTest(ctx, in); err != nil {
					fmt.Println("{Public} DNS + A RECORD test failed ==================== ")
					tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
					return err
				}

				fmt.Println("{Public} DNS + A RECORD TEST FINISHED SUCCESSFULLY, CLEARING SERVICE ANNOTATIONS :))))")
				lgr.Info("\n ======== Public ipv4 test finished successfully, clearing service annotations ======== \n")
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)

				return nil
			},
		},
		{
			name: "public cluster + public DNS +  Quad A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
			run: func(ctx context.Context) error {
				lgr := logger.FromContext(ctx)
				if err := AAAARecordTest(ctx, in); err != nil {
					fmt.Println("{Public} DNS + Aaaa RECORD test failed ==================== ")
					tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
					tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv6Service.Name)
					return err
				}
				fmt.Println("{Public} DNS + Aaaa RECORD TEST FINISHED SUCCESSFULLY, CLEARING SERVICE ANNOTATIONS :))))")
				lgr.Info("\n ======== Public ipv6 test finished successfully, clearing service annotations ======== \n")
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
	ipv4ServiceName := infra.Ipv4ServiceName

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", tests.PublicZone, ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service with zone name", err)
		return fmt.Errorf("error: %s", err)
	}

	//checking to see if A record was created in Azure DNS
	time.Sleep(50 * time.Second)
	err = validateRecord(ctx, armdns.RecordTypeA, resourceGroup, subId, *tests.ClusterName, tests.PublicZone, 50, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		//attempting to delete record set here in case record was created after 50 seconds
		err = tests.DeleteRecordSet(ctx, *tests.ClusterName, subId, resourceGroup, tests.PublicZone, armdns.RecordTypeA)
		if err != nil {
			lgr.Error("Error deleting A record set")
			return fmt.Errorf("error deleting A record set")
		}
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeA)
	} else {
		lgr.Info("Test Passed: Public dns + A record")
	}

	err = tests.DeleteRecordSet(ctx, *tests.ClusterName, subId, resourceGroup, tests.PublicZone, armdns.RecordTypeA)
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

	err = tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", publicZone, ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	//TODO: try removing time.Sleeps everywhere and see if test still passes, otherwise, increase time in loop (param)
	time.Sleep(20 * time.Second)

	//Checking Azure DNS for AAAA record
	err = validateRecord(ctx, armdns.RecordTypeAAAA, resourceGroup, subId, *tests.ClusterName, publicZone, 50, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)

	if err != nil {
		return fmt.Errorf("AAAA Record not created in Azure DNS")
	} else {
		lgr.Info("Test Passed: public dns + AAAA record test")
	}

	err = tests.DeleteRecordSet(ctx, *tests.ClusterName, subId, resourceGroup, publicZone, armdns.RecordTypeA)
	if err != nil {
		lgr.Error("Error deleting A record set")
		return fmt.Errorf("error deleting A record set")
	}
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

	err := tests.WaitForExternalDns(ctx, 10, subscriptionId, rg, clusterName)

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

	timeout := time.Now().Add(numSeconds * time.Second)

	var pageValue []*armdns.RecordSet

	for {

		if time.Now().After(timeout) {
			return fmt.Errorf("record not created within %s seconds", numSeconds)
		}

		pager := clientFactory.NewRecordSetsClient().NewListByTypePager(rg, serviceDnsZoneName, recordType, &armdns.RecordSetsClientListByTypeOptions{Top: nil,
			Recordsetnamesuffix: nil,
		})

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
		time.Sleep(2 * time.Second)
	}

	var ipAddr string
	for _, v := range pageValue {
		currZoneName := strings.Trim(*(v.Properties.Fqdn), ".") //removing trailing '.'

		if recordType == armdns.RecordTypeA {
			ipAddr = *(v.Properties.ARecords[0].IPv4Address)
		} else if recordType == armdns.RecordTypeAAAA {
			ipAddr = *(v.Properties.AaaaRecords[0].IPv6Address)
		} else {
			return fmt.Errorf("unable to match record type")
		}

		if currZoneName == serviceDnsZoneName && ipAddr == svcIp {
			lgr.Info(" ======================== public dns === Record matched!!! ==================== ")
			return nil
		}

	}
	//test failed
	return fmt.Errorf("record not created %s", recordType)

}

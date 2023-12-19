package suites

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	"github.com/Azure/azure-provider-external-dns-e2e/tests"
)

func privateDnsSuite(in infra.Provisioned) []test {
	return []test{
		{
			name: "public cluster + private DNS +  A Record",
			run: func(ctx context.Context) error {
				lgr := logger.FromContext(ctx)
				if err := PrivateARecordTest(ctx, in, false); err != nil {
					fmt.Println("PRIVATE DNS A FAILED ==================== ")
					tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
					return err
				}
				fmt.Println("PRIVATE DNS A RECORD TEST FINISHED SUCCESSFULLY, CLEARING SERVICE ANNOTATIONS :))))")
				lgr.Info("\n ======== Private ipv4 test finished successfully, clearing service annotations ======== \n")
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
				return nil
			},
		},
		{
			name: "public cluster + private DNS +  AAAA Record",
			run: func(ctx context.Context) error {
				lgr := logger.FromContext(ctx)
				if err := PrivateAAAATest(ctx, in, false); err != nil {
					fmt.Println("PRIVATE DNS Aaaa FAILED ==================== ")
					tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
					tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv6Service.Name)
					return err
				}
				fmt.Println("PRIVATE DNS + Aaaa RECORD TEST FINISHED SUCCESSFULLY, CLEARING SERVICE ANNOTATIONS :))))")
				lgr.Info("\n ======== Private ipv6 test finished successfully, clearing service annotations ======== \n ")
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv6Service.Name)
				return nil
			},
		},
	}
}

var PrivateARecordTest = func(ctx context.Context, infra infra.Provisioned, usePublicZone bool) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	//Currently only provisioning one public and one private zone, no test in this suite tests with more than one of each
	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId
	privateZone := infra.PrivateZones[0].GetName()
	ipv4ServiceName := infra.Ipv4ServiceName

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", privateZone, infra.Ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	err = tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", privateZone, ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service with zone name", err)
		return fmt.Errorf("error: %s", err)
	}

	time.Sleep(20 * time.Second)
	//Validating Records
	err = validatePrivateRecords(ctx, armprivatedns.RecordTypeA, resourceGroup, subId, *tests.ClusterName, privateZone, 20, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Private Record not created in Azure DNS", armdns.RecordTypeA)
	} else {
		lgr.Info("Finished test: private dns + A record test successfully")
	}

	err = tests.DeletePrivateRecordSet(ctx, *tests.ClusterName, subId, resourceGroup, privateZone, armprivatedns.RecordTypeA)
	if err != nil {
		lgr.Error("Error deleting AAAA record set")
		return fmt.Errorf("error deleting AAAA record set")
	}

	return nil
}

var PrivateAAAATest = func(ctx context.Context, infra infra.Provisioned, usePublicZone bool) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId

	privateZone := infra.PrivateZones[0].GetName()
	ipv6ServiceName := infra.Ipv6ServiceName

	//TODO: send all of these annotations to Annotate service in a map
	tests.PrivateDnsAnnotations(ctx, subId, *tests.ClusterName, resourceGroup, ipv6ServiceName)

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", privateZone, ipv6ServiceName)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	time.Sleep(20 * time.Second)
	//Validating records
	err = validatePrivateRecords(ctx, armprivatedns.RecordTypeAAAA, resourceGroup, subId, *tests.ClusterName, privateZone, 20, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Private Record not created in Azure DNS", armdns.RecordTypeAAAA)
	} else {
		lgr.Info("Finished test: private dns + AAAA record test successfully")
	}

	//deleting A and AAAA record sets
	err = tests.DeletePrivateRecordSet(ctx, *tests.ClusterName, subId, resourceGroup, privateZone, armprivatedns.RecordTypeA)
	if err != nil {
		lgr.Error("Error deleting A record set")
		return fmt.Errorf("error deleting A record set")
	}
	err = tests.DeletePrivateRecordSet(ctx, *tests.ClusterName, subId, resourceGroup, privateZone, armprivatedns.RecordTypeAAAA)
	if err != nil {
		lgr.Error("Error deleting AAAA record set")
		return fmt.Errorf("error deleting AAAA record set")
	}

	return nil

}

func validatePrivateRecords(ctx context.Context, recordType armprivatedns.RecordType, rg, subscriptionId, clusterName, serviceDnsZoneName string, numSeconds time.Duration, svcIp string) error {
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

	clientFactory, err := armprivatedns.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		log.Fatal(err)
	}

	timeout := time.Now().Add(numSeconds * time.Second)
	var pageValue []*armprivatedns.RecordSet
	for {
		if time.Now().After(timeout) {
			return fmt.Errorf("record not created within %s seconds", numSeconds)
		}
		pager := clientFactory.NewRecordSetsClient().NewListByTypePager(rg, serviceDnsZoneName, recordType, &armprivatedns.RecordSetsClientListByTypeOptions{Top: nil,
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
	}

	var ipAddr string

	for _, v := range pageValue {

		if err != nil {
			log.Fatalf("failed to advance page: %v", err)
			return fmt.Errorf("failed to advance page for record sets")
		}
		currZoneName := strings.Trim(*(v.Properties.Fqdn), ".") //removing trailing '.'

		if recordType == armprivatedns.RecordTypeA {
			ipAddr = *(v.Properties.ARecords[0].IPv4Address)
		} else if recordType == armprivatedns.RecordTypeAAAA {
			ipAddr = *(v.Properties.AaaaRecords[0].IPv6Address)
		} else {
			return fmt.Errorf("unable to match record type")
		}

		if currZoneName == serviceDnsZoneName && ipAddr == svcIp {
			lgr.Info(" ======================== Record matched!!! ==================== ")
			return nil
		}

	}

	return fmt.Errorf("record not created %s", recordType) //test failed
}

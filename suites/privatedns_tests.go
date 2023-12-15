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

	log.Printf("In private dns suite <<<<<<<<<<<<<<<<<<<<<<<<<<<")
	return []test{
		{
			name: "public cluster + private DNS +  A Record",
			run: func(ctx context.Context) error {
				fmt.Println("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
				fmt.Println("Test private DNS + A record")
				fmt.Println("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
				//
				if err := PrivateARecordTest(ctx, in); err != nil {
					fmt.Println("BAD A  private ======================= ")
					tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
					return err
				}
				tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)

				return nil
			},
		},
		{
			name: "public cluster + private DNS +  AAAA Record",
			run: func(ctx context.Context) error {
				fmt.Println("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
				fmt.Println("Test private DNS + AAAA record")
				fmt.Println("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")

				if err := PrivateAAAARecordTest(ctx, in, false); err != nil {
					tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
					tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv6Service.Name)
					fmt.Println("BAD AAAA private ======================= ")
					return err
				}
				tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
				tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv6Service.Name)

				return nil
			},
		},
	}

}

var PrivateAAAARecordTest = func(ctx context.Context, infra infra.Provisioned, usePublicZone bool) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId

	privateZone := infra.PrivateZones[0].GetName()
	ipv6ServiceName := infra.Ipv6ServiceName

	//Private dns specific annotations
	tests.PrivateDnsAnnotations(ctx, subId, *tests.ClusterName, resourceGroup, infra.Ipv4ServiceName)
	tests.PrivateDnsAnnotations(ctx, subId, *tests.ClusterName, resourceGroup, ipv6ServiceName)

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", privateZone, tests.Ipv4Service.Name)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	err = tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", privateZone, tests.Ipv6Service.Name)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	err = validatePrivateRecords(ctx, armprivatedns.RecordTypeAAAA, resourceGroup, subId, *tests.ClusterName, privateZone, 50, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Private Record not created in Azure DNS", armdns.RecordTypeAAAA)
	} else {
		lgr.Info("finished successfully")
	}

	return nil

}

var PrivateARecordTest = func(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	//Currently only provisioning one public and one private zone, no test in this suite tests with more than one of each
	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId
	//publicZone := infra.Zones[0]
	zoneName := infra.PrivateZones[0].GetName()
	ipv4ServiceName := infra.Ipv4ServiceName

	err := tests.PrivateDnsAnnotations(ctx, subId, *tests.ClusterName, resourceGroup, ipv4ServiceName)

	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	err = tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", zoneName, ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service with zone name", err)
		return fmt.Errorf("error: %s", err)
	}

	err = validatePrivateRecords(ctx, armprivatedns.RecordTypeA, resourceGroup, subId, *tests.ClusterName, zoneName, 30, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Private Record not created in Azure DNS", armdns.RecordTypeA)
	} else {
		lgr.Info("finished successfully")
	}

	fmt.Println("End of A Record Test ============ ")
	return nil
}

func validatePrivateRecords(ctx context.Context, recordType armprivatedns.RecordType, rg, subscriptionId, clusterName, serviceDnsZoneName string, numSeconds time.Duration, svcIp string) error {

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

	clientFactory, err := armprivatedns.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		log.Fatal(err)
	}

	pager := clientFactory.NewRecordSetsClient().NewListByTypePager(rg, serviceDnsZoneName, recordType, &armprivatedns.RecordSetsClientListByTypeOptions{Top: nil,
		Recordsetnamesuffix: nil,
	})

	//TODO: fix timeout warning w/ time.Second
	timeout := time.Now().Add(numSeconds * time.Second)

	var pageValue []*armprivatedns.RecordSet
	for {
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
	}

	var ipAddr string

	//matching record, returns nil error if dns zone name and ip address match
	for _, v := range pageValue {
		fmt.Println("In pageValue ==== record created ====== :))))")
		currZoneName := strings.Trim(*(v.Properties.Fqdn), ".") //removing trailing '.'

		if recordType == armprivatedns.RecordTypeA {
			ipAddr = *(v.Properties.ARecords[0].IPv4Address)
		} else if recordType == armprivatedns.RecordTypeAAAA {
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

	return fmt.Errorf("record not created %s", recordType) //test failed
}

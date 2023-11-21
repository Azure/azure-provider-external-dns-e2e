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

func basicSuite(in infra.Provisioned) []test {

	log.Printf("In basic suite >>>>>>>>>>>>>>>>>>>>>>")
	return []test{

		// {
		// 	name: "public cluster + public DNS +  A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
		// 	run: func(ctx context.Context) error {
		// 		tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), in.Ipv4ServiceName)
		// 		if err := ARecordTest(ctx, in, true); err != nil {
		// 			return err
		// 		}
		// 		tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), in.Ipv4ServiceName)
		// 		return nil
		// 	},
		// },
		// {
		// 	name: "public cluster + public DNS +  Quad A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
		// 	run: func(ctx context.Context) error {
		// 		tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), in.Ipv6ServiceName)
		// 		if err := AAAARecordTest(ctx, in, true); err != nil {
		// 			return err
		// 		}
		// 		tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), in.Ipv6ServiceName)
		// 		return nil
		// 	},
		// },
		// {
		// 	name: "public cluster + private DNS +  A Record",
		// 	run: func(ctx context.Context) error {
		// 		tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), in.Ipv4ServiceName)
		// 		if err := ARecordTest(ctx, in, false); err != nil {
		// 			return err
		// 		}
		// 		tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), in.Ipv4ServiceName)
		// 		return nil
		// 	},
		// },
		// {
		// 	name: "public cluster + private DNS +  AAAA Record",
		// 	run: func(ctx context.Context) error {
		// 		//tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), in.Ipv6ServiceName)
		// 		if err := AAAARecordTest(ctx, in, false); err != nil {
		// 			return err
		// 		}
		// 		tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), in.Ipv6ServiceName)
		// 		return nil
		// 	},
		// },

		//private cluster tests with above combinations
	}
}

var AAAARecordTest = func(ctx context.Context, infra infra.Provisioned, usePublicZone bool) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId
	publicZone := infra.Zones[0]
	privateZone := infra.PrivateZones[0]
	ipv6ServiceName := infra.Ipv6ServiceName

	var zoneName string
	if usePublicZone {
		zoneName = publicZone.GetName()
	} else {
		zoneName = privateZone.GetName()
		tests.PrivateDnsAnnotations(ctx, subId, *tests.ClusterName, resourceGroup, ipv6ServiceName)
	}

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", zoneName, "nginx-svc-ipv6")
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	if usePublicZone {
		err = validateRecord(ctx, armdns.RecordTypeAAAA, resourceGroup, subId, *tests.ClusterName, zoneName, 20, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)
		if err != nil {
			return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeAAAA)
		} else {
			lgr.Info("finished successfully")
		}
	} else {
		err = validatePrivateRecords(ctx, armprivatedns.RecordTypeAAAA, resourceGroup, subId, *tests.ClusterName, zoneName, 4, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)
		if err != nil {
			return fmt.Errorf("%s Private Record not created in Azure DNS", armdns.RecordTypeAAAA)
		} else {
			lgr.Info("finished successfully")
		}
	}

	return nil

}

var ARecordTest = func(ctx context.Context, infra infra.Provisioned, usePublicZone bool) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	//Currently only provisioning one public and one private zone, no test in this suite tests with more than one of each
	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId
	publicZone := infra.Zones[0]
	privateZone := infra.PrivateZones[0]
	ipv4ServiceName := infra.Ipv4ServiceName

	// //printing values for debugging
	// fmt.Println("Infra subId: ", infra.SubscriptionId)
	// fmt.Println("Infra cluster name: ", *tests.ClusterName)
	// fmt.Println("Infra rg: ", infra.ResourceGroup.GetName())
	// fmt.Println("Infra zone name: ", publicZone.GetName())
	// fmt.Println("ipv4 Infra Service name: ", infra.Ipv4ServiceName)

	var zoneName string
	if usePublicZone {
		zoneName = publicZone.GetName()
	} else {
		zoneName = privateZone.GetName()
		tests.PrivateDnsAnnotations(ctx, subId, *tests.ClusterName, resourceGroup, ipv4ServiceName)
	}

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", zoneName, ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service with zone name", err)
		return fmt.Errorf("error: %s", err)
	}

	if usePublicZone {
		err = validateRecord(ctx, armdns.RecordTypeA, resourceGroup, subId, *tests.ClusterName, zoneName, 4, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
		if err != nil {
			return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeA)
		} else {
			lgr.Info("finished successfully")
		}

	} else {
		err = validatePrivateRecords(ctx, armprivatedns.RecordTypeA, resourceGroup, subId, *tests.ClusterName, zoneName, 4, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
		if err != nil {
			return fmt.Errorf("%s Private Record not created in Azure DNS", armdns.RecordTypeA)
		} else {
			lgr.Info("finished successfully")
		}
	}

	return nil
}

func validatePrivateRecords(ctx context.Context, recordType armprivatedns.RecordType, rg, subscriptionId, clusterName, serviceDnsZoneName string, numSeconds time.Duration, svcIp string) error {
	fmt.Println()
	fmt.Println("#0 In PRIVATE DNS validateRecord() function ---------------------")

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

	timeout := time.Now().Add(numSeconds * time.Second)

	for {
		if time.Now().After(timeout) {
			return fmt.Errorf("record not created within %s seconds", numSeconds)
		}
		if pager.More() { //TODO: modify for pager.NextPage()
			break
		}
	}

	var ipAddr string

	for pager.More() {
		page, err := pager.NextPage(ctx)

		if err != nil {
			log.Fatalf("failed to advance page: %v", err)
			return fmt.Errorf("failed to advance page for record sets")
		}

		for _, v := range page.Value {

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
				fmt.Println(" ======================== Record matched!!! ==================== ")

				return nil
			}

		}

	}
	//test failed
	return fmt.Errorf("record not created %s", recordType)

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

	for {
		if time.Now().After(timeout) {
			return fmt.Errorf("record not created within %s seconds", numSeconds)
		}
		if pager.More() { //TODO: modify for pager.NextPage()
			break
		}
	}

	var ipAddr string
	fmt.Println("Service ip: ", svcIp)
	fmt.Println("Service zone name:", serviceDnsZoneName)
	for pager.More() {
		fmt.Println("#3 In pager ----------------")
		page, err := pager.NextPage(ctx)

		if err != nil {
			log.Fatalf("failed to advance page: %v", err)
			return fmt.Errorf("failed to advance page for record sets")
		}
		fmt.Println("Page.value length: ", len(page.Value))
		for _, v := range page.Value {
			fmt.Println()
			fmt.Println("In Loop!  dns record created ======================= :)")
			fmt.Println()

			currZoneName := strings.Trim(*(v.Properties.Fqdn), ".") //removing trailing '.'

			if recordType == armdns.RecordTypeA {
				ipAddr = *(v.Properties.ARecords[0].IPv4Address)
			} else if recordType == armdns.RecordTypeAAAA {
				ipAddr = *(v.Properties.AaaaRecords[0].IPv6Address)
			} else {
				return fmt.Errorf("unable to match record type")
			}

			// TODO: change for ipv6 addr
			fmt.Println("#4 =========== Ip address: ", ipAddr)
			fmt.Println("#5 =========== Zone name: ", currZoneName)

			if currZoneName == serviceDnsZoneName && ipAddr == svcIp {
				fmt.Println()
				fmt.Println(" ======================== Record matched!!! ==================== ")

				return nil
			}

		}

	}
	//test failed
	return fmt.Errorf("record not created %s", recordType)

}

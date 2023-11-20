package suites

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns"
	corev1 "k8s.io/api/core/v1"

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

		// 		if err := ARecordTest(ctx, in, true); err != nil {
		// 			return err
		// 		}

		// 		return nil
		// 	},
		// },
		// {
		// 	name: "public cluster + public DNS +  Quad A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
		// 	run: func(ctx context.Context) error {

		// 		if err := AAAARecordTest(ctx, in, corev1.IPFamilyPolicyRequireDualStack, true); err != nil {
		// 			return err
		// 		}

		// 		return nil
		// 	},
		// },

		{
			name: "public cluster + private DNS +  A Record",
			run: func(ctx context.Context) error {

				if err := ARecordTest(ctx, in, false); err != nil {
					return err
				}

				return nil
			},
		},
		// {
		// 	//public cluster + private DNS + AAAA
		// },

		//private cluster tests with above combinations
	}
}

var AAAARecordTest = func(ctx context.Context, infra infra.Provisioned, ipFamilyPolicy corev1.IPFamilyPolicy, usePublicZone bool) error {

	fmt.Println("In AAAA test -- 2 ******************************* ")
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId
	publicZone := infra.Zones[0]
	privateZone := infra.PrivateZones[0]

	var zoneName string
	if usePublicZone {
		zoneName = publicZone.GetName()
	} else {
		zoneName = privateZone.GetName()
	}

	fmt.Printf("annotating ipv6 service ---------- value: %s", publicZone.GetName())
	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", zoneName, "nginx-svc-ipv6")
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	fmt.Println("About to call Validate Record --------------------------")

	err = validateRecord(ctx, armdns.RecordTypeAAAA, resourceGroup, subId, *tests.ClusterName, zoneName, 20, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeAAAA)
	} else {
		lgr.Info("finished successfully")
	}

	return nil

}

var ARecordTest = func(ctx context.Context, infra infra.Provisioned, usePublicZone bool) error {
	fmt.Printf("%+v\n", infra)

	fmt.Println("In A record test --------------")
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	//Currently only provisioning one public and one private zone, no test in this suite tests with more than one of each
	resourceGroup := infra.ResourceGroup.GetName()
	subId := infra.SubscriptionId
	publicZone := infra.Zones[0]
	privateZone := infra.PrivateZones[0]
	ipv4ServiceName := infra.Ipv4ServiceName

	//printing values for debug
	fmt.Println("Infra subId: ", infra.SubscriptionId)
	fmt.Println("Infra cluster name: ", *tests.ClusterName)
	fmt.Println("Infra rg: ", infra.ResourceGroup.GetName())
	fmt.Println("Infra zone name: ", publicZone.GetName())
	fmt.Println("ipv4 Infra Service name: ", infra.Ipv4ServiceName)

	var zoneName string
	if usePublicZone {
		zoneName = publicZone.GetName()
	} else {
		zoneName = privateZone.GetName()
		privateDnsAnnotations(ctx, subId, *tests.ClusterName, resourceGroup, ipv4ServiceName)
	}

	err := tests.AnnotateService(ctx, subId, *tests.ClusterName, resourceGroup, "external-dns.alpha.kubernetes.io/hostname", zoneName, ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service with zone name", err)
		return fmt.Errorf("error: %s", err)
	}

	// //TODO: remove this
	// fmt.Println("------------------ :O Sleeping after annotating service :O---------------------- ")
	// time.Sleep(20 * time.Second)

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
			fmt.Println("In Loop! dns record created ======================= :)")
			fmt.Println()

			currZoneName := strings.Trim(*(v.Properties.Fqdn), ".") //removing trailing '.'

			if recordType == armprivatedns.RecordTypeA {
				ipAddr = *(v.Properties.ARecords[0].IPv4Address)
			} else if recordType == armprivatedns.RecordTypeAAAA {
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

// Checks to see whether record is created in Azure DNS
func validateRecord(ctx context.Context, recordType armdns.RecordType, rg, subscriptionId, clusterName, serviceDnsZoneName string, numSeconds time.Duration, svcIp string) error {
	fmt.Println()
	fmt.Println("#0 In validateRecord() function ---------------------")

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

// adds annotations needed specifically for private dns tests
func privateDnsAnnotations(ctx context.Context, subId, clusterName, rg, serviceName string) error {

	lgr := logger.FromContext(ctx)
	lgr.Info("Adding annotations for private dns")

	err := tests.AnnotateService(ctx, subId, clusterName, rg, "service.beta.kubernetes.io/azure-load-balancer-internal", "true", serviceName)
	if err != nil {
		lgr.Error("Error annotating service to create internal load balancer ", err)
		return fmt.Errorf("error: %s", err)
	}

	err = tests.AnnotateService(ctx, subId, clusterName, rg, "external-dns.alpha.kubernetes.io/internal-hostname", "server-clusterip.example.com", serviceName)
	if err != nil {
		lgr.Error("Error annotating service for private dns", err)
		return fmt.Errorf("error: %s", err)
	}

	return nil

}

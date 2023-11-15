package suites

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	corev1 "k8s.io/api/core/v1"

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

				if err := ARecordTest(ctx, in, true); err != nil {
					return err
				}

				return nil
			},
		},
		// {
		// 	name: "public cluster + public DNS +  Quad A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
		// 	run: func(ctx context.Context) error {

		// 		if err := AAAARecordTest(ctx, in, corev1.IPFamilyPolicyRequireDualStack, true); err != nil {
		// 			return err
		// 		}

		// 		return nil
		// 	},
		// },

		// {
		// 	name: "public cluster + private DNS +  A Record",
		// 	run: func(ctx context.Context) error {

		// 		if err := ARecordTest(ctx, in, false); err != nil {
		// 			return err
		// 		}

		// 		return nil
		// 	},
		// },
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

	err := tests.AddIPFamilySpec(ctx, infra, tests.Ipv6Service, ipFamilyPolicy, usePublicZone)
	if err != nil {
		return fmt.Errorf("Error adding ip family and ip family policy to service spec and upserting: %s", err)
	}

	//TODO: assuming here theres only one public zone
	publicZone := infra.Zones[0]
	privateZone := infra.PrivateZones[0]

	fmt.Println("annotating ipv6 service ----------")
	err = tests.AnnotateService(ctx, infra.SubscriptionId, *tests.ClusterName, infra.ResourceGroup.GetName(), "external-dns.alpha.kubernetes.io/hostname", publicZone.GetName(), "nginx-svc-ipv6")
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	var zoneName string
	if usePublicZone {
		zoneName = publicZone.GetName()
	} else {
		zoneName = privateZone.GetName()
	}
	fmt.Println("About to call Validate Record --------------------------")
	//TODO: change public zone param here
	err = validateRecord(ctx, armdns.RecordTypeAAAA, tests.ResourceGroup, tests.SubscriptionId, *tests.ClusterName, zoneName, 20, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)
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
	publicZone := infra.Zones[0]
	privateZone := infra.PrivateZones[0]

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
	}

	err := tests.AnnotateService(ctx, infra.SubscriptionId, *tests.ClusterName, infra.ResourceGroup.GetName(), "external-dns.alpha.kubernetes.io/hostname", zoneName, infra.Ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	err = validateRecord(ctx, armdns.RecordTypeA, infra.ResourceGroup.GetName(), infra.SubscriptionId, *tests.ClusterName, zoneName, 4, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeA)
	} else {
		lgr.Info("finished successfully")
	}

	return nil
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

	//Private record sets client?
	pager := clientFactory.NewRecordSetsClient().NewListByTypePager(rg, serviceDnsZoneName, recordType, &armdns.RecordSetsClientListByTypeOptions{Top: nil,
		Recordsetnamesuffix: nil,
	})

	timeout := time.Now().Add(numSeconds * time.Second)
	for {
		if time.Now().After(timeout) {
			return fmt.Errorf("Record not created within %s seconds", numSeconds)
		}
		if pager.More() {
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
	return fmt.Errorf("record not created %s", armdns.RecordTypeA)

}

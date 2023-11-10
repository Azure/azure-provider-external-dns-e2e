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

				if err := ARecordTest(ctx, in, true); //func(service *corev1.Service) error {

				err != nil {
					return err
				}

				return nil
			},
		},
		// {
		// 	name: "public cluster + public DNS +  Quad A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
		// 	run: func(ctx context.Context) error {

		// 		if err := AAAARecordTest(ctx, in, tests.Ipv6, corev1.IPFamilyPolicyRequireDualStack, true); //func(service *corev1.Service) error {

		// 		err != nil {
		// 			return err
		// 		}

		// 		return nil
		// 	},
		// },
		// {
		// 	//public cluster _ public DNS + CNAME
		// },
		// {
		// 	//public cluster + public DNS + MX
		// },
		// {
		// 	//public cluster + public DNS + TXT
		// },
		// {
		// 	//public cluster + private DNS + A
		// 	name: "public cluster + private DNS +  A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
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
		// {
		// 	//public cluster + private DNS + CNAME
		// },
		// {
		// 	//public cluster + private DNS + MX
		// },
		// {
		// 	//public cluster + private DNS + TXT
		// },
		// --Private Cluster Tests with same combinations as above
	}
}

var AAAARecordTest = func(ctx context.Context, infra infra.Provisioned, recordType tests.IpFamily, ipFamilyPolicy corev1.IPFamilyPolicy, usePublicZone bool) error {

	fmt.Println("In AAAA test -- 2 -----------------------------")
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	//convert record type to ipfamily enum type
	err := tests.AddIPFamilySpec(ctx, infra, recordType, ipFamilyPolicy, usePublicZone)
	if err != nil {
		return fmt.Errorf("Error adding ip family and ip family policy to service spec and upserting: %s", err)
	}

	publicZone := infra.Zones[0]
	fmt.Println("About to call Validate Record --------------------------")
	err = validateRecord(ctx, armdns.RecordTypeAAAA, tests.ResourceGroup, tests.SubscriptionId, *tests.ClusterName, publicZone.GetName(), 4, tests.Service.Status.LoadBalancer.Ingress[0].IP)
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
	clusterName := tests.ClusterName

	//printing values for debug
	fmt.Println("Infra subId: ", infra.SubscriptionId)
	fmt.Println("Infra cluster name: ", *clusterName)
	fmt.Println("Infra rg: ", infra.ResourceGroup.GetName())
	fmt.Println("Infra zone name: ", publicZone.GetName())
	fmt.Println("Infra Service name: ", infra.ServiceName)

	err := tests.AnnotateService(ctx, infra.SubscriptionId, *clusterName, infra.ResourceGroup.GetName(), "external-dns.alpha.kubernetes.io/hostname", publicZone.GetName(), infra.ServiceName)
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
	err = validateRecord(ctx, armdns.RecordTypeA, infra.ResourceGroup.GetName(), infra.SubscriptionId, *clusterName, zoneName, 4, tests.Service.Status.LoadBalancer.Ingress[0].IP)
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

	timeout := time.Now().Add(7 * time.Second)
	for {
		if time.Now().After(timeout) {
			return fmt.Errorf("Record not created within %s seconds", numSeconds)
		}
		if pager.More() {
			break
		}
	}

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
			fmt.Println("In Loop!  dns record created ======================= :)")

			//TODO: Switch/ case for every type of dns record

			currZoneName := strings.Trim(*(v.Properties.Fqdn), ".") //removing trailing '.'
			ipAddr := *(v.Properties.ARecords[0].IPv4Address)       // TODO: change for ipv6 addr
			fmt.Println("#4 =========== Ip address: ", ipAddr)
			fmt.Println("#5 =========== Zone name: ", currZoneName)

			if currZoneName == serviceDnsZoneName && ipAddr == svcIp {
				fmt.Println()
				fmt.Println(" ======================== Record matched!!! ====================")

				return nil
			}

		}

	}
	//test failed
	return fmt.Errorf("record not created %s", armdns.RecordTypeA)

}

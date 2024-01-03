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

// Tests using the provisioned public dns zone for creating A and AAAA records
func basicSuite(in infra.Provisioned) []test {
	return []test{
		{
			name: "public DNS +  A Record",
			run: func(ctx context.Context) error {
				lgr := logger.FromContext(ctx)

				if err := ARecordTest(ctx, in); err != nil {
					tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
					return err
				}
				lgr.Info("\n ======== Public Dns ipv4 test finished successfully, clearing service annotations ======== \n")
				tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
				return nil
			},
		},
		{
			name: "public DNS +  Quad A Record",
			run: func(ctx context.Context) error {
				lgr := logger.FromContext(ctx)
				if err := AAAARecordTest(ctx, in); err != nil {
					tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
					tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv6Service.Name)
					return err
				}
				lgr.Info("\n ======== Public Dns ipv6 test finished successfully, clearing service annotations ======== \n")
				tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
				tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv6Service.Name)

				return nil
			},
		},
	}
}

var ARecordTest = func(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting public dns + A record test")

	ipv4ServiceName := infra.Ipv4ServiceName

	annotationMap := map[string]string{
		"external-dns.alpha.kubernetes.io/hostname": tests.PublicZone,
	}
	err := tests.AnnotateService(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, ipv4ServiceName, annotationMap)
	if err != nil {
		lgr.Error("Error annotating service with zone name", err)
		return fmt.Errorf("error: %s", err)
	}

	//checking to see if A record was created in Azure DNS
	err = validateRecord(ctx, armdns.RecordTypeA, tests.ResourceGroup, tests.SubId, *tests.ClusterName, tests.PublicZone, 150, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeA)
	} else {
		lgr.Info("Test Passed: Public dns + A record")
	}

	//test passed, deleting created record set
	err = tests.DeleteRecordSet(ctx, *tests.ClusterName, tests.SubId, tests.ResourceGroup, tests.PublicZone, armdns.RecordTypeA, "")
	if err != nil {
		lgr.Error("Error deleting A record set")
		return fmt.Errorf("error deleting A record set")
	}
	return nil
}

var AAAARecordTest = func(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting public dns + AAAA test")

	ipv6ServiceName := infra.Ipv6ServiceName
	ipv4ServiceName := infra.Ipv4ServiceName

	annotationMap := map[string]string{
		"external-dns.alpha.kubernetes.io/hostname": tests.PublicZone,
	}

	err := tests.AnnotateService(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, ipv6ServiceName, annotationMap)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	err = tests.AnnotateService(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, ipv4ServiceName, annotationMap)
	if err != nil {
		lgr.Error("Error annotating service", err)
		return fmt.Errorf("error: %s", err)
	}

	// Checking Azure DNS for AAAA record
	err = validateRecord(ctx, armdns.RecordTypeAAAA, tests.ResourceGroup, tests.SubId, *tests.ClusterName, tests.PublicZone, 100, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)

	if err != nil {
		return fmt.Errorf("AAAA Record not created in Azure DNS")
	} else {
		lgr.Info("Test Passed: public dns + AAAA record test")
	}

	// Test passed, deleting created record sets
	err = tests.DeleteRecordSet(ctx, *tests.ClusterName, tests.SubId, tests.ResourceGroup, tests.PublicZone, armdns.RecordTypeA, "")
	if err != nil {
		lgr.Error("Error deleting A record set")
		return fmt.Errorf("error deleting A record set")
	}
	err = tests.DeleteRecordSet(ctx, *tests.ClusterName, tests.SubId, tests.ResourceGroup, tests.PublicZone, armdns.RecordTypeAAAA, "")
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

	err := tests.WaitForExternalDns(ctx, 10, subscriptionId, rg, clusterName, "external-dns")
	if err != nil {
		return fmt.Errorf("error waiting for ExternalDNS to start running %w", err)
	}

	cred, err := clients.GetAzCred()
	if err != nil {
		return fmt.Errorf("getting az credentials: %w", err)
	}

	clientFactory, err := armdns.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		log.Fatal("failed to create client: ", err)
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
				log.Fatal("failed to advance page: ", err)
				return fmt.Errorf("failed to advance page for record sets")
			}
			if len(page.Value) > 0 {
				pageValue = page.Value
				break
			}

		}
		time.Sleep(2 * time.Second) //waiting 2 seconds before checking again
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
			return nil
		}

	}
	//test failed
	return fmt.Errorf("record not created %s", recordType)

}

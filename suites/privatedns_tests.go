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

// Tests using the provisioned private dns zone for creating A and AAAA records
func privateDnsSuite(in infra.Provisioned) []test {
	return []test{
		{
			name: "private DNS +  A Record",
			run: func(ctx context.Context) error {
				lgr := logger.FromContext(ctx)
				if err := PrivateARecordTest(ctx, in); err != nil {
					tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
					return err
				}
				lgr.Info("\n ======== Private Dns ipv4 test finished successfully, clearing service annotations ======== \n")
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
				return nil
			},
		},
		{
			name: "private DNS +  AAAA Record",
			run: func(ctx context.Context) error {
				lgr := logger.FromContext(ctx)
				if err := PrivateAAAATest(ctx, in); err != nil {
					tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
					tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv6Service.Name)
					return err
				}
				lgr.Info("\n ======== Private Dns ipv6 test finished successfully, clearing service annotations ======== \n ")
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv4Service.Name)
				tests.ClearAnnotations(ctx, in.SubscriptionId, *tests.ClusterName, in.ResourceGroup.GetName(), tests.Ipv6Service.Name)
				return nil
			},
		},
	}
}

var PrivateARecordTest = func(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	ipv4ServiceName := infra.Ipv4ServiceName

	err := tests.PrivateDnsAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, ipv4ServiceName)
	if err != nil {
		lgr.Error("Error annotating service with private dns annotations", err)
		return fmt.Errorf("error: %s", err)
	}

	//Validating Records
	err = validatePrivateRecords(ctx, armprivatedns.RecordTypeA, tests.ResourceGroup, tests.SubId, *tests.ClusterName, tests.PrivateZone, 150, tests.Ipv4Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Private Record not created in Azure DNS", armdns.RecordTypeA)
	} else {
		lgr.Info("Test Passed: Private Dns + A record test successfully")
	}

	err = tests.DeleteRecordSet(ctx, *tests.ClusterName, tests.SubId, tests.ResourceGroup, tests.PrivateZone, "", armprivatedns.RecordTypeA)
	if err != nil {
		lgr.Error("Error deleting AAAA record set")
		return fmt.Errorf("error deleting AAAA record set")
	}

	return nil
}

var PrivateAAAATest = func(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	ipv6ServiceName := infra.Ipv6ServiceName

	err := tests.PrivateDnsAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, ipv6ServiceName)
	if err != nil {
		lgr.Error("Error annotating service with private dns annotations", err)
		return fmt.Errorf("error: %s", err)
	}

	//Validating records
	err = validatePrivateRecords(ctx, armprivatedns.RecordTypeAAAA, tests.ResourceGroup, tests.SubId, *tests.ClusterName, tests.PrivateZone, 150, tests.Ipv6Service.Status.LoadBalancer.Ingress[0].IP)
	if err != nil {
		return fmt.Errorf("%s Private Record not created in Azure DNS", armdns.RecordTypeAAAA)
	} else {
		lgr.Info("Test Passed: Private Dns + AAAA record test successfully")
	}

	//Deleting A and AAAA record sets
	err = tests.DeleteRecordSet(ctx, *tests.ClusterName, tests.SubId, tests.ResourceGroup, tests.PrivateZone, "", armprivatedns.RecordTypeAAAA)
	if err != nil {
		lgr.Error("Error deleting AAAA record set")
		return fmt.Errorf("error deleting AAAA record set")
	}

	return nil

}

func validatePrivateRecords(ctx context.Context, recordType armprivatedns.RecordType, rg, subscriptionId, clusterName, serviceDnsZoneName string, numSeconds time.Duration, svcIp string) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("Checking that Record was created in Azure DNS")

	//Default 10 seconds to wait for external dns pod to start running, can be modified in the future if needed
	err := tests.WaitForExternalDns(ctx, 10, subscriptionId, rg, clusterName, "external-dns-private")
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
				log.Fatal("failed to advance page", err)
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

		if recordType == armprivatedns.RecordTypeA {
			ipAddr = *(v.Properties.ARecords[0].IPv4Address)
		} else if recordType == armprivatedns.RecordTypeAAAA {
			ipAddr = *(v.Properties.AaaaRecords[0].IPv6Address)
		} else {
			return fmt.Errorf("unable to match record type")
		}

		if currZoneName == serviceDnsZoneName && ipAddr == svcIp {
			return nil
		}

	}

	return fmt.Errorf("record not created %s", recordType) //test failed
}

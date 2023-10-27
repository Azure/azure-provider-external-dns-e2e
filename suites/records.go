package suites

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

var cred azcore.TokenCredential

// This file contains wrapper functions for obtaining record sets used in testing (A, AAAA, CNAME, MX, TXT)
func init() {
	creds, err := getAzCred()
	if err != nil {
		fmt.Errorf("getting az credentials: %w", err)
	}
	cred = creds
}
func createARecord(ctx context.Context, subId, rg, dnsZone, relativeName, ipAddr string) (*armdns.ARecord, error) {

	lgr := logger.FromContext(ctx)
	lgr.Info("In createARecord Function >>>>>>>>>>>>>>>>>>>>>>>>>>>>")
	lgr.Info("starting to get A record")
	defer lgr.Info("finished getting A record")

	clientFactory, err := armdns.NewClientFactory(subId, cred, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	res, err := clientFactory.NewRecordSetsClient().CreateOrUpdate(ctx, rg, dnsZone, relativeName, armdns.RecordTypeA, armdns.RecordSet{
		Properties: &armdns.RecordSetProperties{
			ARecords: []*armdns.ARecord{
				{
					IPv4Address: to.Ptr(ipAddr), //"127.0.0.1" sample ip address
				}},
			// TTL: to.Ptr[int64](3600),
			// Metadata: map[string]*string{
			// 	"key1": to.Ptr("value1"),
			// },
		},
	}, &armdns.RecordSetsClientCreateOrUpdateOptions{IfMatch: nil,
		IfNoneMatch: nil,
	})
	if err != nil {
		log.Fatalf("failed to finish the request: %v", err)
	}

	//solid way to check for 200 and 201 response?
	return res.RecordSet.Properties.ARecords[0], nil
}

func getQuadARecord(ctx context.Context, subId, rg, dnsZone, relativeName, ipAddr string) (*armdns.ARecord, error) {

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("failed to obtain a credential: %v", err)
	}

	clientFactory, err := armdns.NewClientFactory("<subscription-id>", cred, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	res, err := clientFactory.NewRecordSetsClient().CreateOrUpdate(ctx, "rg1", "zone1", "record1", armdns.RecordTypeAAAA, armdns.RecordSet{
		Properties: &armdns.RecordSetProperties{
			AaaaRecords: []*armdns.AaaaRecord{
				{
					IPv6Address: to.Ptr("::1"),
				}},
			TTL: to.Ptr[int64](3600),
			Metadata: map[string]*string{
				"key1": to.Ptr("value1"),
			},
		},
	}, &armdns.RecordSetsClientCreateOrUpdateOptions{IfMatch: nil,
		IfNoneMatch: nil,
	})
	if err != nil {
		log.Fatalf("failed to finish the request: %v", err)
	}
	// You could use response here. We use blank identifier for just demo purposes.
	_ = res
	// If the HTTP response code is 200 as defined in example definition, your response structure would look as follows. Please pay attention that all the values in the output are fake values for just demo purposes.
	// res.RecordSet = armdns.RecordSet{
	// 	Name: to.Ptr("record1"),
	// 	Type: to.Ptr("Microsoft.Network/dnsZones/AAAA"),
	// 	Etag: to.Ptr("00000000-0000-0000-0000-000000000000"),
	// 	ID: to.Ptr("/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/dnsZones/zone1/AAAA/record1"),
	// 	Properties: &armdns.RecordSetProperties{
	// 		AaaaRecords: []*armdns.AaaaRecord{
	// 			{
	// 				IPv6Address: to.Ptr("::1"),
	// 		}},
	// 		TTL: to.Ptr[int64](3600),
	// 		Fqdn: to.Ptr("record1.zone1"),
	// 		Metadata: map[string]*string{
	// 			"key1": to.Ptr("value1"),
	// 		},
	// 	},
	// }
	return nil, nil
}

func getCNAMERecord() {

}

func getMXRecord() {

}

func getTXTRecord() {

}

// Retrieves same credentials from auth through az cli
func getAzCred() (azcore.TokenCredential, error) {
	if cred != nil {
		return cred, nil
	}

	c, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("getting az cli credential: %w", err)
	}

	cred = c
	return cred, nil
}

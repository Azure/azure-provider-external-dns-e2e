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

func getARecord(ctx context.Context, rg, dnsZone, relativeName, ipAddr string) (*armdns.ARecord, error) {

	lgr := logger.FromContext(ctx)
	lgr.Info("starting to get A record")
	defer lgr.Info("finished getting A record")

	cred, err := GetAzCred()
	if err != nil {
		return nil, fmt.Errorf("getting az credentials: %w", err)
	}

	clientFactory, err := armdns.NewClientFactory("<subscription-id>", cred, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	res, err := clientFactory.NewRecordSetsClient().CreateOrUpdate(ctx, rg, dnsZone, relativeName, armdns.RecordTypeA, armdns.RecordSet{
		Properties: &armdns.RecordSetProperties{
			ARecords: []*armdns.ARecord{
				{
					IPv4Address: to.Ptr(ipAddr), //"127.0.0.1" sample ip address
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

	//solid way to check for 200 and 201 response?
	return res.RecordSet.Properties.ARecords[0], nil
}

func getQuadARecord() {

}

func getCNAMERecord() {

}

func getMXRecord() {

}

func getTXTRecord() {

}

// Retrieves same credentials from auth through az cli
func GetAzCred() (azcore.TokenCredential, error) {
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

package suites

import (
	"context"
	"fmt"
	"log"

	"github.com/go-errors/errors"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
)

// TODO: add init function to set creds, might not make sense
// var cred azcore.TokenCredential

func createARecord() {

	ctx := context.Background()
	lgr := logger.FromContext(ctx)

	//cred, err := getAzCred() //todo: error handler
	creds, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		lgr.Info("#0 failed to obtain a credential: %v", err)
	}

	//lgr.Info("#1 Before creating client factory")
	clientFactory, err := armdns.NewClientFactory("8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8", creds, nil)
	if err != nil {
		lgr.Info("#1.5 failed to create client: %v", err)
		log.Fatalf("failed to create client: %v", err)
	}

	//lgr.Info("#2 Before calling create and update on new record set client")
	client := clientFactory.NewRecordSetsClient()

	res, err := client.CreateOrUpdate(ctx, "test-rg-msc", "msctestdnszone.com", "record3", armdns.RecordTypeA, armdns.RecordSet{
		Properties: &armdns.RecordSetProperties{
			ARecords: []*armdns.ARecord{
				{
					IPv4Address: to.Ptr("127.0.0.1"),
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
		lgr.Error("#4 failed to finish the request: %v", err)
		//fmt.Errorf("failed to finish the request: %w", err)
		fmt.Println(err.(*errors.Error).ErrorStack())
		//log.Fatalf("failed to finish the request: %w", err)
	}
	lgr.Info("#5 After error not nil check")

	// You could use response here. We use blank identifier for just demo purposes.
	_ = res
	// If the HTTP response code is 200 as defined in example definition, your response structure would look as follows. Please pay attention that all the values in the output are fake values for just demo purposes.
	// res.RecordSet = armdns.RecordSet{
	// 	Name: to.Ptr("record1"),
	// 	Type: to.Ptr("Microsoft.Network/dnsZones/A"),
	// 	Etag: to.Ptr("00000000-0000-0000-0000-000000000000"),
	// 	ID: to.Ptr("/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/dnsZones/zone1/A/record1"),
	// 	Properties: &armdns.RecordSetProperties{
	// 		ARecords: []*armdns.ARecord{
	// 			{
	// 				IPv4Address: to.Ptr("127.0.0.1"),
	// 		}},
	// 		TTL: to.Ptr[int64](3600),
	// 		Fqdn: to.Ptr("record1.zone1"),
	// 		Metadata: map[string]*string{
	// 			"key1": to.Ptr("value1"),
	// 		},
	// 	},
	// }
}

// func getAzCred() (azcore.TokenCredential, error) {
// 	if cred != nil {
// 		return cred, nil
// 	}

// 	c, err := azidentity.NewAzureCLICredential(nil)
// 	if err != nil {
// 		return nil, fmt.Errorf("getting az cli credential: %w", err)
// 	}

// 	cred = c
// 	return cred, nil
// }

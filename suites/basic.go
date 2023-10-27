package suites

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/client-go/rest"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	"github.com/Azure/azure-provider-external-dns-e2e/manifests"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func basicSuite(in infra.Provisioned) []test {

	ctx := context.Background()
	lgr := logger.FromContext(ctx).With("name", "basicSuite")
	ctx = logger.WithContext(ctx, lgr)

	log.Printf("In basic suite >>>>>>>>>>>>>>>>>>>>>>")
	lgr.Info("IN BASIC SUITE >>>>>>>>>>>>>>>>>>>>>>>>>")
	return []test{

		{
			name: "public + A Record", //public cluster + public DNS + A Record TODO: set naming convention for all tests
			run: func(ctx context.Context, config *rest.Config) error {

				//TODO: generate random IP address for testing instead of using 127.0.0.1,
				//TODO: change dns relative zone name, standardize
				_, err := createARecord(ctx, in.SubscriptionId, in.ResourceGroup.GetName(), in.Zones[0].GetName(), "dnsRelativeName", "127.0.0.1")

				if err != nil {
					lgr.Error("error getting A Record")
					return fmt.Errorf("error getting A Record: %s", err)
				}
				//remove ingress param from modifier func, maybe zoner too?
				if err := basicRecordTest(ctx, config, in, func(service *corev1.Service) error {
					//TODO: service is currently nil, save ObjectMeta and corev1.ServiceSpec to infra-config.json?
					annotations := service.GetAnnotations()
					publicDNSZoneName := in.Zones[0].GetName()
					annotations["external-dns.alpha.kubernetes.io/hostname"] = publicDNSZoneName
					service.SetAnnotations(annotations) //in-memory,need to save
					//upsert done in test function below
					return nil
				}); err != nil {
					return err
				}

				return nil
			},
		},
	}
}

// modifier is a function that can be used to modify the ingress and service
type modifier func(service *corev1.Service) error

var basicRecordTest = func(ctx context.Context, config *rest.Config, infra infra.Provisioned, mod modifier) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting test")

	//TODO: Is this client JUST needed to upsert? What does this do?
	c, err := client.New(config, client.Options{})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}
	publicZone := infra.Zones[0]

	//service obj
	serviceObj := manifests.GetNginxServiceForTesting()

	//updates service with annotation
	if mod != nil {
		if err := mod(serviceObj); err != nil {
			return fmt.Errorf("modifying service: %w", err)
		}
	}

	//Upserting service obj
	if err := upsert(ctx, c, serviceObj); err != nil {
		return fmt.Errorf("upserting service, %w", err)
	}

	//TODO: change record type A to armDNS record type const
	err = validateRecord(armdns.RecordTypeA, infra.ResourceGroup.GetName(), infra.SubscriptionId, publicZone.GetName(), 2, *serviceObj)
	if err != nil {
		return fmt.Errorf("%s Record not created in Azure DNS", armdns.RecordTypeA)
	}
	lgr.Info("finished successfully")
	return nil
}

// Checks to see whether record is created in Azure DNS
// time out value param
func validateRecord(recordType armdns.RecordType, rg, subscriptionId, dnsZoneName string, timeout time.Duration, service corev1.Service) error {
	//TODO: timeout loop to wait on external dns
	//for loop that tests to see if
	//checks every timeout interval if external dns is done, once it's up and running, it exits

	cred, err := getAzCred()
	if err != nil {
		return fmt.Errorf("getting az credentials: %w", err)
	}

	ctx := context.Background()

	clientFactory, err := armdns.NewClientFactory(subscriptionId, cred, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	pager := clientFactory.NewRecordSetsClient().NewListByTypePager(rg, dnsZoneName, recordType, &armdns.RecordSetsClientListByTypeOptions{Top: nil,
		Recordsetnamesuffix: nil,
	})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Fatalf("failed to advance page: %v", err)
		}
		for _, v := range page.Value {
			//fmt.Println("Record type: ", *(v.Type))
			fmt.Println("Ip address: ", *(v.Properties.ARecords[0]))

			//grab a dns record who's dns zone name matches the service dns zone name. Then check to see if the ip address matches the
			//ip address on the load balancer

			//checking one Value at a time?
			// if *(v.Type) != "Microsoft.Network/dnsZones/A" {
			// 	return fmt.Errorf("A record not created in Azure DNS, test failed")
			// }

			//if (v.Properties.ARecords[0][IPv4Address]) -- TODO: check IP addr. Figure out what else we should check

		}

	}

	return nil
}

type zoner interface {
	GetName() string
	GetNameserver() string
}

type zone struct {
	name       string
	nameserver string
}

func (z zone) GetName() string {
	return z.name
}

func (z zone) GetNameserver() string {
	return z.nameserver
}

// If the HTTP response code is 200 as defined in example definition, your page structure would look as follows. Please pay attention that all the values in the output are fake values for just demo purposes.
// page.RecordSetListResult = armdns.RecordSetListResult{
// 	Value: []*armdns.RecordSet{
// 		{
// 			Name: to.Ptr("record1"),
// 			Type: to.Ptr("Microsoft.Network/dnsZones/A"),
// 			Etag: to.Ptr("00000000-0000-0000-0000-000000000000"),
// 			ID: to.Ptr("/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/dnsZones/zone1/A/record1"),
// 			Properties: &armdns.RecordSetProperties{
// 				ARecords: []*armdns.ARecord{
// 					{
// 						IPv4Address: to.Ptr("127.0.0.1"),
// 				}},
// 				TTL: to.Ptr[int64](3600),
// 				Fqdn: to.Ptr("record1.zone1"),
// 				Metadata: map[string]*string{
// 					"key1": to.Ptr("value1"),
// 				},
// 			},
// 	}},
// }

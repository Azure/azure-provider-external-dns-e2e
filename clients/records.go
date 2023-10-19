package clients

import (
	"context"
)

//This file contains wrapper functions for obtaining record sets used in testing (A, AAAA, CNAME, MX, TXT)

func getArecord(ctx context.Context) {

	// lgr := logger.FromContext(ctx).With("name", name, "subscriptionId", subscriptionId, "resourceGroup", resourceGroup)
	// ctx = logger.WithContext(ctx, lgr)
	// lgr.Info("starting to get dns")
	// defer lgr.Info("finished getting dns")

	// cred, err := getAzCred()
	// if err != nil {
	// 	return nil, fmt.Errorf("getting az credentials: %w", err)
	// }

	// client, err := armdns.NewRecordSetsClient(z.subscriptionId, cred, nil)
	// if err != nil {
	// 	return nil, fmt.Errorf("creating client: %w", err)
	// }

	// resp, err := client.Get(ctx, z.resourceGroup, z.name, nil)
	// if err != nil {
	// 	return nil, fmt.Errorf("getting dns: %w", err)
	// }

	// return &resp.Zone, nil
}

func getQuadARecord() {

}

func getCNAMERecord() {

}

func getMXRecord() {

}

func getTXTRecord() {

}

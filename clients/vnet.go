package clients

//code obtained from azure-sdk-for-go-samples/sdk/resourcemanager/network/networkInterface

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

var (
	subscriptionID     string
	resourceGroupName  string
	location           string
	virtualNetworkName = "sample-virtual-network"
	subnetName         = "sample-subnet"

	virtualNetworksClient *armnetwork.VirtualNetworksClient
	subnetsClient         *armnetwork.SubnetsClient
	networkClientFactory  *armnetwork.ClientFactory
)

func NewVnet(ctx context.Context, subId, rg, region, privateZoneName string) (string, string, error) {
	subscriptionID = subId
	resourceGroupName = rg
	location = region

	cred, err := GetAzCred()
	if err != nil {
		return "", "", fmt.Errorf("getting az credentials: %w", err)
	}

	networkClientFactory, err = armnetwork.NewClientFactory(subscriptionID, cred, nil)
	if err != nil {
		log.Fatal(err)
	}

	virtualNetworksClient = networkClientFactory.NewVirtualNetworksClient()
	subnetsClient = networkClientFactory.NewSubnetsClient()

	virtualNetwork, err := createVirtualNetwork(ctx)
	if err != nil {
		log.Fatal(err)
	}

	subnet, err := createSubnet(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return *virtualNetwork.ID, *subnet.ID, nil
}

func createVirtualNetwork(ctx context.Context) (*armnetwork.VirtualNetwork, error) {
	pollerResp, err := virtualNetworksClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		virtualNetworkName,
		armnetwork.VirtualNetwork{
			Location: to.Ptr(location),
			Properties: &armnetwork.VirtualNetworkPropertiesFormat{
				AddressSpace: &armnetwork.AddressSpace{
					AddressPrefixes: []*string{
						to.Ptr("fd00:db8:deca::/48"),
						to.Ptr("10.1.0.0/16"),
					},
				},
			},
		},
		nil)

	if err != nil {
		return nil, err
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.VirtualNetwork, nil
}

func createSubnet(ctx context.Context) (*armnetwork.Subnet, error) {
	pollerResp, err := subnetsClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		virtualNetworkName,
		subnetName,
		armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefixes: []*string{
					to.Ptr("fd00:db8:deca:deed::/64"),
					to.Ptr("10.1.0.0/24"),
				},
			},
		},
		nil)

	if err != nil {
		return nil, err
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.Subnet, nil
}

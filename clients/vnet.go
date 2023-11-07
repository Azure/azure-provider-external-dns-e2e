package clients

//code obtained from azure-sdk-for-go-samples/sdk/resourcemanager/network/networkInterface

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

// TODO: change values here and test
var (
	subscriptionID    string
	resourceGroupName string
	location          string
	// networkInterfaceName = "sample-network-interface"
	virtualNetworkName = "sample-virtual-network"
	subnetName         = "sample-subnet"
	// publicIPAddressName  = "sample-public-ip"
	// securityGroupName    = "sample-network-security-group"
)

var (
	networkClientFactory *armnetwork.ClientFactory
)

var (
	virtualNetworksClient *armnetwork.VirtualNetworksClient
	subnetsClient         *armnetwork.SubnetsClient
	// publicIPAddressesClient *armnetwork.PublicIPAddressesClient
	// securityGroupsClient    *armnetwork.SecurityGroupsClient
	// interfacesClient        *armnetwork.InterfacesClient
)

func NewVnet(ctx context.Context, subId, rg, region string) (string, string, error) {

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
	// publicIPAddressesClient = networkClientFactory.NewPublicIPAddressesClient()
	// securityGroupsClient = networkClientFactory.NewSecurityGroupsClient()
	// interfacesClient = networkClientFactory.NewInterfacesClient()

	virtualNetwork, err := createVirtualNetwork(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("virtual network:", *virtualNetwork.ID)

	subnet, err := createSubnet(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("subnet:", *subnet.ID)

	// //TODO: need this?
	// publicIP, err := createPublicIP(ctx)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Println("public ip:", *publicIP.ID)

	// //TODO: need this?
	// networkSecurityGroup, err := createNetworkSecurityGroup(ctx)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Println("network security group:", *networkSecurityGroup.ID)

	// //TODO: need this?
	// nic, err := createNIC(ctx, *subnet.ID, *publicIP.ID, *networkSecurityGroup.ID)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Println("network interface:", *nic.ID)

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

// func createPublicIP(ctx context.Context) (*armnetwork.PublicIPAddress, error) {

// 	pollerResp, err := publicIPAddressesClient.BeginCreateOrUpdate(
// 		ctx,
// 		resourceGroupName,
// 		publicIPAddressName,
// 		armnetwork.PublicIPAddress{
// 			Name:     to.Ptr(publicIPAddressName),
// 			Location: to.Ptr(location),
// 			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
// 				PublicIPAddressVersion:   to.Ptr(armnetwork.IPVersionIPv4),
// 				PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
// 			},
// 		},
// 		nil,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	resp, err := pollerResp.PollUntilDone(ctx, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &resp.PublicIPAddress, nil
// }

// func createNetworkSecurityGroup(ctx context.Context) (*armnetwork.SecurityGroup, error) {

// 	pollerResp, err := securityGroupsClient.BeginCreateOrUpdate(
// 		ctx,
// 		resourceGroupName,
// 		securityGroupName,
// 		armnetwork.SecurityGroup{
// 			Location: to.Ptr(location),
// 			Properties: &armnetwork.SecurityGroupPropertiesFormat{
// 				SecurityRules: []*armnetwork.SecurityRule{
// 					{
// 						Name: to.Ptr("allow_ssh"),
// 						Properties: &armnetwork.SecurityRulePropertiesFormat{
// 							Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
// 							SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
// 							SourcePortRange:          to.Ptr("1-65535"),
// 							DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
// 							DestinationPortRange:     to.Ptr("22"),
// 							Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
// 							Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
// 							Priority:                 to.Ptr[int32](100),
// 						},
// 					},
// 					{
// 						Name: to.Ptr("allow_https"),
// 						Properties: &armnetwork.SecurityRulePropertiesFormat{
// 							Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
// 							SourceAddressPrefix:      to.Ptr("0.0.0.0/0"),
// 							SourcePortRange:          to.Ptr("1-65535"),
// 							DestinationAddressPrefix: to.Ptr("0.0.0.0/0"),
// 							DestinationPortRange:     to.Ptr("443"),
// 							Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
// 							Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
// 							Priority:                 to.Ptr[int32](200),
// 						},
// 					},
// 				},
// 			},
// 		},
// 		nil)

// 	if err != nil {
// 		return nil, err
// 	}

// 	resp, err := pollerResp.PollUntilDone(ctx, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &resp.SecurityGroup, nil
// }

// func createNIC(ctx context.Context, subnetID, publicIPID, networkSecurityGroupID string) (*armnetwork.Interface, error) {

// 	pollerResp, err := interfacesClient.BeginCreateOrUpdate(
// 		ctx,
// 		resourceGroupName,
// 		networkInterfaceName,
// 		armnetwork.Interface{
// 			Location: to.Ptr(location),
// 			Properties: &armnetwork.InterfacePropertiesFormat{
// 				IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
// 					{
// 						Name: to.Ptr("ipConfig"),
// 						Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
// 							PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
// 							Subnet: &armnetwork.Subnet{
// 								ID: to.Ptr(subnetID),
// 							},
// 							PublicIPAddress: &armnetwork.PublicIPAddress{
// 								ID: to.Ptr(publicIPID),
// 							},
// 						},
// 					},
// 				},
// 				NetworkSecurityGroup: &armnetwork.SecurityGroup{
// 					ID: to.Ptr(networkSecurityGroupID),
// 				},
// 			},
// 		},
// 		nil,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	resp, err := pollerResp.PollUntilDone(ctx, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &resp.Interface, nil
// }

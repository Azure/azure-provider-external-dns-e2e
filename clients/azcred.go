package clients

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

var cred azcore.TokenCredential

// Uses NewAzureCLICredential, returns creds used to provision all infrastructure and create clients used in tests
func GetAzCred() (azcore.TokenCredential, error) {
	if cred != nil {
		return cred, nil
	}

	// this is CLI instead of DefaultCredential to ensure we are using the same credential as the CLI
	// and authed through the cli. We use the az cli directly when pushing an image to ACR for now.
	c, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("getting az cli credential: %w", err)
	}

	cred = c
	return cred, nil
}

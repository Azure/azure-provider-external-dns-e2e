package infra

import (
	"github.com/google/uuid"

	"github.com/Azure/azure-provider-external-dns-e2e/clients"
)

// TODO: change rg prefix once tests are done - not urgent
var (
	rg              = "sample3-routing-e2e" + uuid.New().String()
	location        = "westus"
	publicZoneName  = "public-zone-" + uuid.NewString()
	privateZoneName = "private-zone-" + uuid.NewString()
)

// Infras is a list of infrastructure configurations the e2e tests will run against
var Infras = infras{
	{
		Name:          "basic cluster",
		ResourceGroup: rg,
		Location:      location,
		Suffix:        uuid.New().String(),
	},
	{
		Name:          "private cluster",
		ResourceGroup: rg,
		Location:      location,
		Suffix:        uuid.New().String(),
		McOpts:        []clients.McOpt{clients.PrivateClusterOpt},
	},
}

func (i infras) FilterNames(names []string) infras {
	ret := infras{}
	for _, infra := range i {
		for _, name := range names {
			if infra.Name == name {
				ret = append(ret, infra)
				break
			}
		}
	}

	return ret
}

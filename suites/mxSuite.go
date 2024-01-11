package suites

import (
	"context"
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	"github.com/Azure/azure-provider-external-dns-e2e/tests"
)

// Tests using the provisioned public dns zone for creating A and AAAA records
func mxSuite(in infra.Provisioned) []test {
	return []test{
		{
			name: "public DNS +  A Record",
			run: func(ctx context.Context) error {
				lgr := logger.FromContext(ctx)

				if err := MxRecordTest(ctx, in); err != nil {
					tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
					return err
				}
				lgr.Info("\n ======== Public Dns MX record test finished successfully, clearing service annotations ======== \n")
				tests.ClearAnnotations(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, tests.Ipv4Service.Name)
				return nil
			},
		},
	}
}

var MxRecordTest = func(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("starting public dns + A record test")

	ipv4ServiceName := infra.Ipv4ServiceName

	annotationMap := map[string]string{
		"external-dns.alpha.kubernetes.io/hostname": "mail.example.com",
	}
	err := tests.AnnotateService(ctx, tests.SubId, *tests.ClusterName, tests.ResourceGroup, ipv4ServiceName, annotationMap)
	if err != nil {
		lgr.Error("Error annotating service with zone name", err)
		return fmt.Errorf("error: %s", err)
	}
	return nil
}

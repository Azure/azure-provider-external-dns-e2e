package suites

import (
	"azure-provider-external-dns-e2e/tests"
	"context"
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"k8s.io/client-go/rest"
)

// All returns all tests in all suites
// The infra parameter is infrastructure unmarshaled from the Provision step
func All(infra infra.Provisioned) tests.Ts {
	t := []test{}
	t = append(t, basicSuite(infra)...)

	ret := make(tests.Ts, len(t))
	for i, t := range t {
		ret[i] = t
	}

	return ret
}

type test struct {
	name string
	//cfgs operatorCfgs
	run func(ctx context.Context, config *rest.Config) error
}

func (t test) GetName() string {
	return t.name
}

// func (t test) GetOperatorConfigs() []manifests.OperatorConfig {
// 	return t.cfgs
// }

func (t test) Run(ctx context.Context, config *rest.Config) error {
	if t.run == nil {
		return fmt.Errorf("no run function provided for test %s", t.GetName())
	}

	return t.run(ctx, config)
}

var alwaysRun = func(infra infra.Provisioned) bool {
	return true
}

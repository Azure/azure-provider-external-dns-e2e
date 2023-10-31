package suites

import (
	"context"
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/tests"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
)

// All returns all test in all suites
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
	run  func(ctx context.Context) error
}

func (t test) GetName() string {
	return t.name
}

func (t test) Run(ctx context.Context) error {
	if t.run == nil {
		return fmt.Errorf("no run function provided for test %s", t.GetName())
	}

	return t.run(ctx)
}

var alwaysRun = func(infra infra.Provisioned) bool {
	return true
}

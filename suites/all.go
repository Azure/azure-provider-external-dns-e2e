package suites

import (
	"context"
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-provider-external-dns-e2e/tests"
)

// All returns all test in all suites
func All(infra infra.Provisioned) []tests.Ts {

	//Add new testing suites here:
	var allSuites [][]test
	allSuites = append(allSuites, basicSuite(infra))
	allSuites = append(allSuites, privateDnsSuite(infra))

	final := make([]tests.Ts, len(allSuites))

	for _, suite := range allSuites {
		ret := make(tests.Ts, len(suite))
		for j, w := range suite {
			ret[j] = w
		}
		final = append(final, ret)
	}

	return final
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

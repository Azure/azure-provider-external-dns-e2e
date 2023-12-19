package suites

import (
	"context"
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"
	"github.com/Azure/azure-provider-external-dns-e2e/tests"
)

var (
	suiteCount = 0
)

// All returns all test in all suites
func All(infra infra.Provisioned) []tests.Ts {

	//TODO: standardize for any number of suites
	suiteCount = 2

	fmt.Println("suite count: ", suiteCount)
	t1 := []test{}
	t2 := []test{}
	t1 = append(t1, basicSuite(infra)...)
	t2 = append(t2, privateDnsSuite(infra)...)

	ret0 := make(tests.Ts, 2)
	ret1 := make(tests.Ts, 2)
	for i, t := range t1 {
		fmt.Println("appending test: ", t.GetName())
		ret0[i] = t
	}

	for i, t := range t2 {
		fmt.Println("appending test: ", t.GetName())
		ret1[i] = t
	}

	final := make([]tests.Ts, 2)
	final[0] = ret0
	final[1] = ret1

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

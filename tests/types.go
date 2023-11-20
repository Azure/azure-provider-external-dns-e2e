package tests

import (
	"context"
)

type test interface {
	GetName() string
	Run(ctx context.Context) error
}

// T is an interface for a single test
type T interface {
	// GetOperatorConfigs returns a slice of OperatorConfig structs that should be used for this test.
	// All OperatorConfigs that are compatible should be returned.
	//GetOperatorConfigs() []manifests.OperatorConfig
	test
}

// Ts is a slice of T
type Ts []T

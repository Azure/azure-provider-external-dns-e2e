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
	test
}

// Ts is a slice of T
type Ts []T

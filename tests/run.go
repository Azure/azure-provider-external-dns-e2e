package tests

import (
	"context"
	"fmt"

	"github.com/Azure/azure-provider-external-dns-e2e/infra"

	"github.com/Azure/azure-provider-external-dns-e2e/logger"
	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func init() {
	log.SetLogger(logr.New(log.NullLogSink{})) // without this controller-runtime panics. We use it solely for the client so we can ignore logs
}

func (allTests Ts) Run(ctx context.Context, infra infra.Provisioned) error {
	lgr := logger.FromContext(ctx)
	lgr.Info("In RUN FUNCTION >>>>>>>>>>>>>>>>>>>>>>>>>>")

	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("getting in-cluster config: %w", err)
	}

	runTestFn := func(t test, ctx context.Context) *logger.LoggedError {
		lgr := logger.FromContext(ctx).With("test", t.GetName())
		ctx = logger.WithContext(ctx, lgr)
		lgr.Info("starting to run test")

		if err := t.Run(ctx, config); err != nil {
			return logger.Error(lgr, err)
		}

		lgr.Info("finished running test")
		return nil
	}

	publicZones := make([]string, len(infra.Zones))
	for i, zone := range infra.Zones {
		publicZones[i] = zone.GetId()
	}
	privateZones := make([]string, len(infra.PrivateZones))
	for i, zone := range infra.PrivateZones {
		privateZones[i] = zone.GetId()
	}

	//Loop to run ALL Tests
	lgr.Info("starting to run tests")

	var eg errgroup.Group
	for _, t := range allTests {
		func(t test) {
			eg.Go(func() error {
				if err := runTestFn(t, ctx); err != nil {
					return fmt.Errorf("running test: %w", err)
				}

				return nil
			})
		}(t)
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	lgr.Info("successfully finished running tests")
	return nil
}

func keys[T comparable, V any](m map[T]V) []T {
	ret := make([]T, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}

	return ret
}

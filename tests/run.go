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
	lgr.Info("determining testing order")
	// ordered := t.order(ctx)
	// if len(ordered) == 0 {
	// 	return errors.New("no tests to run")
	// }

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

	// JUST LOGGING run strategy
	// for i, runStrategy := range ordered {
	// 	lgr.Info("run strategy testing order",
	// 		"index", i,
	// 		"operatorVersion", runStrategy.config.Version.String(),
	// 		"operatorDeployStrategy", runStrategy.operatorDeployStrategy.string(),
	// 		"privateZones", runStrategy.config.Zones.Private.String(),
	// 		"publicZones", runStrategy.config.Zones.Public.String(),
	// 		"disableOsm", runStrategy.config.DisableOsm,
	// 	)
	// }

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

// order builds the testing order for the given tests
// func (t Ts) order(ctx context.Context) ordered {
// 	lgr := logger.FromContext(ctx)
// 	// group tests by operator version
// 	lgr.Info("grouping tests by operator version")
// 	operatorVersionSet := make(map[manifests.OperatorVersion][]testWithConfig)
// 	for _, test := range t {
// 		for _, config := range test.GetOperatorConfigs() {
// 			withConfig := testWithConfig{
// 				test:   test,
// 				config: config,
// 			}
// 			operatorVersionSet[config.Version] = append(operatorVersionSet[config.Version], withConfig)
// 		}
// 	}

// 	// order operator versions in ascending order
// 	versions := keys(operatorVersionSet)
// 	sort.Slice(versions, func(i, j int) bool {
// 		return versions[i] < versions[j]
// 	})

// 	if len(versions) == 0 { // would mean no tests were supplied
// 		return nil
// 	}
// 	if versions[len(versions)-1] != manifests.OperatorVersionLatest { // this should be impossible
// 		panic("operatorVersionLatest should always be the last version in the sorted versions")
// 	}

// 	// combine tests that use the same operator configuration and operator version, so they can run in parallel
// 	lgr.Info("grouping tests by operator configuration")
// 	ret := make(ordered, 0)
// 	for _, version := range versions {
// 		// group tests by operator configuration
// 		operatorCfgSet := make(map[manifests.OperatorConfig][]testWithConfig)
// 		for _, test := range operatorVersionSet[version] {
// 			operatorCfgSet[test.config] = append(operatorCfgSet[test.config], test)
// 		}

// 		testsForVersion := make([]testsWithRunInfo, 0)
// 		for cfg, tests := range operatorCfgSet {
// 			var casted []test
// 			for _, test := range tests {
// 				casted = append(casted, test.test)
// 			}

// 			testsForVersion = append(testsForVersion, testsWithRunInfo{
// 				tests:                  casted,
// 				config:                 cfg,
// 				operatorDeployStrategy: upgrade,
// 			})
// 		}
// 		ret = append(ret, testsForVersion...)

// 		// operatorVersionLatest should always be the last version in the sorted versions
// 		if version == manifests.OperatorVersionLatest {
// 			// need to add cleanDeploy tests for the latest version (this is the version we are testing)
// 			new := make([]testsWithRunInfo, 0, len(testsForVersion))
// 			for _, tests := range testsForVersion {
// 				new = append(new, testsWithRunInfo{
// 					tests:                  tests.tests,
// 					config:                 tests.config,
// 					operatorDeployStrategy: cleanDeploy,
// 				})
// 			}
// 			ret = append(ret, new...)
// 		}
// 	}

// 	return ret
// }

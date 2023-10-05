package testutils

import (
	"testing"

	"github.com/Azure/azure-provider-external-dns-e2e/pkg/controller/controllername"
	"github.com/Azure/azure-provider-external-dns-e2e/pkg/controller/metrics"
	promDTO "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func GetErrMetricCount(t *testing.T, controllerName controllername.ControllerNamer) float64 {
	errMetric, err := metrics.AppRoutingReconcileErrors.GetMetricWithLabelValues(controllerName.MetricsName())
	require.NoError(t, err)

	metricProto := &promDTO.Metric{}

	err = errMetric.Write(metricProto)
	require.NoError(t, err)

	beforeCount := metricProto.GetCounter().GetValue()
	return beforeCount
}

func GetReconcileMetricCount(t *testing.T, controllerName controllername.ControllerNamer, label string) float64 {
	errMetric, err := metrics.AppRoutingReconcileTotal.GetMetricWithLabelValues(controllerName.MetricsName(), label)
	require.NoError(t, err)

	metricProto := &promDTO.Metric{}

	err = errMetric.Write(metricProto)
	require.NoError(t, err)

	beforeCount := metricProto.GetCounter().GetValue()
	return beforeCount
}

func StartTestingEnv() (*rest.Config, *envtest.Environment, error) {
	env := &envtest.Environment{}
	restConfig, err := env.Start()
	if err != nil {
		return nil, nil, err
	}
	return restConfig, env, nil
}

func CleanupTestingEnv(env *envtest.Environment) error {
	return env.Stop()
}

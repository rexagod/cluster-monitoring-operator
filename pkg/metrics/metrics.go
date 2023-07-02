package metrics

import (
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

// ReconcileAttempts is a counter that indicates the number of attempts to reconcile the operator configuration.
var ReconcileAttempts = metrics.NewCounter(&metrics.CounterOpts{
	Name:           "cluster_monitoring_operator_reconcile_attempts_total",
	Help:           "Number of attempts to reconcile the operator configuration.",
	StabilityLevel: metrics.ALPHA,
})

// ReconcileStatus is a gauge that indicates the latest reconciliation state.
var ReconcileStatus = metrics.NewGauge(&metrics.GaugeOpts{
	Name:           "cluster_monitoring_operator_last_reconciliation_successful",
	Help:           "Latest reconciliation state. Set to 1 if last reconciliation succeeded, else 0.",
	StabilityLevel: metrics.ALPHA,
})

// ConfiguredCollectionProfile is a gauge that indicates the configured collection profile.
// All collection profiles, as stated in manifests.ConfiguredCollectionProfile, are present as label values at all times.
// A metric value of 1 indicates expected behavior, whereas 0 indicates a misconfigured collection profile.
var ConfiguredCollectionProfile = metrics.NewGaugeVec(&metrics.GaugeOpts{
	Name:           "cluster_monitoring_operator_configured_collection_profile",
	Help:           "The configured collection profile.",
	StabilityLevel: metrics.ALPHA,
	// TODO: Use LabelValueAllowLists, once GA'd: https://pkg.go.dev/k8s.io/component-base/metrics#MetricLabelAllowList.
	// Refer (KEP-2305, currently beta): https://github.com/kubernetes/enhancements/tree/master/keps/sig-instrumentation/2305-metrics-cardinality-enforcement#summary.
}, []string{"last_configured_collection_profile"})

func init() {
	// The API (metrics) server is instrumented to work with component-base.
	// Refer: https://github.com/kubernetes/kubernetes/blob/ec87834bae787ab6687921d65c3bcfde8a6e01b9/staging/src/k8s.io/apiserver/pkg/server/routes/metrics.go#L44.
	legacyregistry.MustRegister(ReconcileAttempts)
	legacyregistry.MustRegister(ReconcileStatus)
	legacyregistry.MustRegister(ConfiguredCollectionProfile)
}

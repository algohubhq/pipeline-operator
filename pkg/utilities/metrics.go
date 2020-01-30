package utilities

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Global metrics variables
var (
	PipelineDeploymentCountGuage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pipeline_operator_running_deployments",
		Help: "Total running pipeline deployments",
	})
	AlgoCountGuage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pipeline_operator_running_algos",
		Help: "Total running Algos",
	})
	DataConnectorCountGuage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pipeline_operator_running_dataconnectors",
		Help: "Total running Data Connectors",
	})
	TopicCountGuage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pipeline_operator_topics",
		Help: "Total topics managed by the operator",
	})
)

// ServeCustomMetrics creates the metrics http server
func RegisterCustomMetrics() {

	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(PipelineDeploymentCountGuage, AlgoCountGuage, DataConnectorCountGuage, TopicCountGuage)

}

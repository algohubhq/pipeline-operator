package utilities

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Global metrics variables
var (
	EndpointCountGuage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "endpoint_operator_running_endpoints",
		Help: "Total running Endpoints",
	})
	AlgoCountGuage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "endpoint_operator_running_algos",
		Help: "Total running Algos",
	})
	DataConnectorCountGuage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "endpoint_operator_running_dataconnectors",
		Help: "Total running Data Connectors",
	})
	TopicCountGuage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "endpoint_operator_topics",
		Help: "Total topics managed by the operator",
	})
)

// CustomMetrics creates the operator specific prometheus metrics
type CustomMetrics struct {
	client    client.Client
	namespace string
}

// NewCustomMetrics returns a new AlgoReconciler
func NewCustomMetrics(client client.Client,
	namespace string) CustomMetrics {
	return CustomMetrics{
		client:    client,
		namespace: namespace,
	}
}

func (customMetrics *CustomMetrics) ServeCustomMetrics() {

	prometheus.MustRegister(EndpointCountGuage)
	prometheus.MustRegister(AlgoCountGuage)
	prometheus.MustRegister(DataConnectorCountGuage)
	prometheus.MustRegister(TopicCountGuage)

	http.Handle("/metrics", promhttp.Handler())

	deplUtil := NewDeploymentUtil(customMetrics.client)
	// Generate the custom metrics service
	metricsService, err := customMetrics.createMetricServiceSpec()
	if err != nil {
		log.Error(err, "Failed to create custom metrics service spec")
	}

	err = deplUtil.CreateService(metricsService)
	if err != nil {
		log.Error(err, "Failed to create custom metrics service")
	}

	go func() {
		http.ListenAndServe(":10080", nil)
	}()

}

func (customMetrics *CustomMetrics) createMetricServiceSpec() (*corev1.Service, error) {

	labels := map[string]string{
		"name": "algorun-endpoint-operator",
	}

	metricsServiceSpec := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: customMetrics.namespace,
			Name:      "endpoint-custom-metrics",
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name: "metrics",
					Port: 10080,
				},
			},
			Selector: map[string]string{
				"name": "algorun-endpoint-operator",
			},
		},
	}

	return metricsServiceSpec, nil

}

/*
 * Algo.Run API 1.0-beta1
 *
 * API for the Algo.Run Engine
 *
 * API version: 1.0-beta1
 * Contact: support@algohub.com
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package v1beta1
// DataConnectorDeploymentV1beta1 struct for DataConnectorDeploymentV1beta1
type DataConnectorDeploymentV1beta1 struct {
	DataConnectorRef *CustomResourceRefV1beta1 `json:"dataConnectorRef,omitempty"`
	Version string `json:"version"`
	Index int32 `json:"index,omitempty"`
	TasksMax int32 `json:"tasksMax,omitempty"`
	Topics []TopicConfigModel `json:"topics,omitempty"`
	Options []DataConnectorOptionModel `json:"options,omitempty"`
	Replicas int32 `json:"replicas,omitempty"`
	Resources *ResourceRequirementsV1 `json:"resources,omitempty"`
	Autoscaling *AutoScalingSpec `json:"autoscaling,omitempty"`
	LivenessProbe *ProbeV1 `json:"livenessProbe,omitempty"`
	ReadinessProbe *ProbeV1 `json:"readinessProbe,omitempty"`
	Spec *DataConnectorSpecV1beta1 `json:"spec,omitempty"`
}
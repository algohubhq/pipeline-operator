/*
 * Algo.Run API 1.0
 *
 * API for the Algo.Run Engine
 *
 * API version: 1.0
 * Contact: support@algohub.com
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package v1alpha1

type EndpointConfig struct {

	EndpointOwnerUserName string `json:"endpointOwnerUserName,omitempty"`

	EndpointName string `json:"endpointName,omitempty"`

	PipelineOwnerUserName string `json:"pipelineOwnerUserName,omitempty"`

	PipelineName string `json:"pipelineName,omitempty"`

	PipelineVersionTag string `json:"pipelineVersionTag,omitempty"`

	AlgoConfigs []AlgoConfig `json:"algoConfigs,omitempty"`

	DataConnectorConfigs []DataConnectorConfig `json:"dataConnectorConfigs,omitempty"`

	TopicConfigs []TopicConfigModel `json:"topicConfigs,omitempty"`

	Pipes []PipeModel `json:"pipes,omitempty"`
}

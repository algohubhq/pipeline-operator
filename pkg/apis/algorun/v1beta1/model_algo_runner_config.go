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
// AlgoRunnerConfig struct for AlgoRunnerConfig
type AlgoRunnerConfig struct {
	DeploymentOwner string `json:"deploymentOwner"`
	DeploymentName string `json:"deploymentName"`
	PipelineOwner string `json:"pipelineOwner"`
	PipelineName string `json:"pipelineName"`
	Owner string `json:"owner"`
	Name string `json:"name"`
	Version string `json:"version"`
	Index int32 `json:"index"`
	Entrypoint string `json:"entrypoint,omitempty"`
	Executor *Executors `json:"executor,omitempty"`
	Parameters []AlgoParamSpec `json:"parameters,omitempty"`
	Inputs []AlgoInputSpec `json:"inputs,omitempty"`
	Outputs []AlgoOutputSpec `json:"outputs,omitempty"`
	WriteAllOutputs bool `json:"writeAllOutputs,omitempty"`
	Pipes []PipeModel `json:"pipes,omitempty"`
	Topics []TopicConfigModel `json:"topics,omitempty"`
	RetryEnabled bool `json:"retryEnabled,omitempty"`
	RetryStrategy *TopicRetryStrategyModel `json:"retryStrategy,omitempty"`
	GpuEnabled bool `json:"gpuEnabled,omitempty"`
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

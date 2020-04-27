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
// AlgoSpecV1beta1 struct for AlgoSpecV1beta1
type AlgoSpecV1beta1 struct {
	Owner string `json:"owner"`
	Name string `json:"name"`
	Versions []AlgoVersionModel `json:"versions"`
	Executor *Executors `json:"executor,omitempty"`
	Parameters []AlgoParamSpec `json:"parameters,omitempty"`
	Inputs []AlgoInputSpec `json:"inputs,omitempty"`
	Outputs []AlgoOutputSpec `json:"outputs,omitempty"`
	GpuEnabled bool `json:"gpuEnabled,omitempty"`
}
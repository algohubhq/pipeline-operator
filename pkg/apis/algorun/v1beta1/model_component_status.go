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
// ComponentStatus struct for ComponentStatus
type ComponentStatus struct {
	ComponentType *ComponentTypes `json:"componentType,omitempty"`
	Name string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Index int32 `json:"index,omitempty"`
	DeploymentName string `json:"deploymentName,omitempty"`
	Status string `json:"status,omitempty"`
	Desired int32 `json:"desired,omitempty"`
	Current int32 `json:"current,omitempty"`
	UpToDate int32 `json:"upToDate,omitempty"`
	Available int32 `json:"available,omitempty"`
	Ready int32 `json:"ready,omitempty"`
	CreatedTimestamp string `json:"createdTimestamp,omitempty"`
}

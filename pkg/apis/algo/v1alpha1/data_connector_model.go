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

type DataConnectorModel struct {

	Id int32 `json:"id,omitempty"`

	DataConnectorType string `json:"dataConnectorType,omitempty"`

	Name string `json:"name,omitempty"`

	Description string `json:"description,omitempty"`

	Url string `json:"url,omitempty"`

	ConnectorClass string `json:"connectorClass,omitempty"`

	TasksMax int32 `json:"tasksMax,omitempty"`

	Options []DataConnectorOptionModel `json:"options,omitempty"`
}
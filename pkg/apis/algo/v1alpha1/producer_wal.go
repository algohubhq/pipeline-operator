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

type ProducerWal struct {

	Mode string `json:"mode,omitempty"`

	Path string `json:"path,omitempty"`

	Size string `json:"size,omitempty"`

	AlwaysTopics []string `json:"always_topics,omitempty"`

	DisableTopics []string `json:"disable_topics,omitempty"`
}
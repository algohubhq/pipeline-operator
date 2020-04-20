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
// Int32OrStringV1 struct for Int32OrStringV1
type Int32OrStringV1 struct {
	Int32Value int32 `json:"int32Value,omitempty"`
	StringValue string `json:"stringValue,omitempty"`
	IsInt32 bool `json:"isInt32,omitempty"`
	IsString bool `json:"isString,omitempty"`
}
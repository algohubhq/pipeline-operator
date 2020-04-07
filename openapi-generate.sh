#!/bin/bash

# Local instance must be running to pull the swagger.json file
java -jar ./openapi-generator-cli-4.2.3.jar generate -i http://localhost:5000/swagger/v1-beta1/swagger.json \
-g go \
-p enumClassPrefix=true \
-p packageName=v1beta1 \
-t openapi-template \
-o algorun-go-client

cp ./algorun-go-client/model_pipeline_deployment_spec_v1beta1.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_algo_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_data_connector_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_algo_runner_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_hook_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_component_status.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_resource_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_scale_metric_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_config_mount_model.go ./pkg/apis/algorun/v1beta1/
# uncomment if algo pod status is updated but there will be manual edits
# cp ./algorun-go-client/model_component_pod_status.go ./pkg/apis/algorun/v1beta1/

cp ./algorun-go-client/model_endpoint_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_endpoint_server_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_endpoint_producer_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_endpoint_kafka_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_endpoint_server_listen.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_producer_circuit_breaker.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_producer_resend.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_producer_wal.go ./pkg/apis/algorun/v1beta1/

cp ./algorun-go-client/model_notif_message.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_deployment_status_message.go ./pkg/apis/algorun/v1beta1/

cp ./algorun-go-client/model_endpoint_path_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_algo_param_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_topic_config_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_topic_param_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_topic_retry_strategy_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_topic_retry_step_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_data_type_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_data_type_option_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_content_type_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_algo_input_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_algo_output_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_pipe_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_web_hook_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_data_connector_option_model.go ./pkg/apis/algorun/v1beta1/

cp ./algorun-go-client/model_retry_strategies.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_log_levels.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_log_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_notif_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_executors.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_input_delivery_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_output_delivery_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_data_connector_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_data_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_component_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_message_data_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_metric_source_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_metric_target_types.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/model_endpoint_types.go ./pkg/apis/algorun/v1beta1/

# find ./pkg/apis/algorun/v1beta1/ -name '*.go' -exec sed -i 's/package swagger/package v1beta1/g' {} \;

rm -rf ./algorun-go-client/

export GOROOT=$(go env GOROOT)

GO111MODULE=on operator-sdk generate k8s
# GO111MODULE=on operator-sdk generate openapi
GO111MODULE=on operator-sdk generate crds
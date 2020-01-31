#!/bin/bash

# Local instance must be running to pull the swagger.json file
java -jar ./swagger-codegen-cli.jar generate -i http://localhost:5000/swagger/v1-beta1/swagger.json -Dio.swagger.parser.util.RemoteUrl.trustAll=true -l go -o algorun-go-client -c ./swagger-config.json

cp ./algorun-go-client/pipeline_spec.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/algo_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/data_connector_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/algo_runner_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/hook_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/component_status.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/resource_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/scale_metric_model.go ./pkg/apis/algorun/v1beta1/
# uncomment if algo pod status is updated but there will be manual edits
# cp ./algorun-go-client/component_pod_status.go ./pkg/apis/algorun/v1beta1/

cp ./algorun-go-client/endpoint_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/endpoint_server_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/endpoint_producer_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/endpoint_kafka_config.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/endpoint_server_listen.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/producer_circuit_breaker.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/producer_resend.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/producer_wal.go ./pkg/apis/algorun/v1beta1/

cp ./algorun-go-client/notif_message.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/deployment_status_message.go ./pkg/apis/algorun/v1beta1/

cp ./algorun-go-client/endpoint_path_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/algo_param_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/topic_config_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/topic_param_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/data_type_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/data_type_option_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/content_type_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/algo_input_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/algo_output_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/pipe_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/web_hook_model.go ./pkg/apis/algorun/v1beta1/
cp ./algorun-go-client/data_connector_option_model.go ./pkg/apis/algorun/v1beta1/

# find ./pkg/apis/algorun/v1beta1/ -name '*.go' -exec sed -i 's/package swagger/package v1beta1/g' {} \;

rm -rf ./algorun-go-client/

export GOROOT=$(go env GOROOT)

GO111MODULE=on operator-sdk generate k8s
# GO111MODULE=on operator-sdk generate openapi
GO111MODULE=on operator-sdk generate crds
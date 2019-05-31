#!/bin/bash

# Local instance must be running to pull the swagger.json file
java -jar ./swagger-codegen-cli.jar generate -i https://localhost:5443/swagger/v1/swagger.json -Dio.swagger.parser.util.RemoteUrl.trustAll=true -l go -o algorun-go-client

cp ./algorun-go-client/endpoint_config.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/algo_config.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/data_connector_config.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/algo_runner_config.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/hook_config.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/hook_runner_config.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/algo_deployment_status.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/algo_pod_status.go ./pkg/apis/algo/v1alpha1/

cp ./algorun-go-client/notif_message.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/endpoint_status_message.go ./pkg/apis/algo/v1alpha1/

cp ./algorun-go-client/algo_param_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/topic_config_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/topic_param_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/data_type_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/data_type_option_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/media_type_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/algo_input_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/algo_output_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/pipe_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/web_hook_model.go ./pkg/apis/algo/v1alpha1/
cp ./algorun-go-client/data_connector_option_model.go ./pkg/apis/algo/v1alpha1/

find ./pkg/apis/algo/v1alpha1/ -name '*.go' -exec sed -i 's/package swagger/package v1alpha1/g' {} \;

rm -rf ./algorun-go-client/


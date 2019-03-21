:: Local instance must be running to pull the swagger.json file

java -jar swagger-codegen-cli.jar generate -i http://localhost:5000/swagger/v1/swagger.json -Dio.swagger.parser.util.RemoteUrl.trustAll=true -l go -o %cd%\algorun-go-client

xcopy %cd%\algorun-go-client\runner_config.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\algo_param_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\topic_config_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\topic_param_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\data_type_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\data_type_option_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\media_type_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\algo_input_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\algo_output_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\pipe_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\pipeline_data_connector_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\data_connector_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\data_connector_option_model.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\algo_config.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\orchestrator_command.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\endpoint_deploy_cmd.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\endpoint_stop_cmd.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\log_message.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\runner_log_data.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\server_log_data.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\algo_log_data.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\orchestrator_log_data.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\endpoint_log_data.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\deployment_log_data.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R
xcopy %cd%\algorun-go-client\instance_log_data.go %cd%\pkg\apis\algo\v1alpha1\types\ /Y /R

rd /s /q %cd%\algorun-go-client


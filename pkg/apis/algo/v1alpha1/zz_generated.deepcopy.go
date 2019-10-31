// +build !ignore_autogenerated

// Code generated by operator-sdk. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlgoConfig) DeepCopyInto(out *AlgoConfig) {
	*out = *in
	if in.AlgoParams != nil {
		in, out := &in.AlgoParams, &out.AlgoParams
		*out = make([]AlgoParamModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Inputs != nil {
		in, out := &in.Inputs, &out.Inputs
		*out = make([]AlgoInputModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Outputs != nil {
		in, out := &in.Outputs, &out.Outputs
		*out = make([]AlgoOutputModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlgoConfig.
func (in *AlgoConfig) DeepCopy() *AlgoConfig {
	if in == nil {
		return nil
	}
	out := new(AlgoConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlgoDeploymentStatus) DeepCopyInto(out *AlgoDeploymentStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlgoDeploymentStatus.
func (in *AlgoDeploymentStatus) DeepCopy() *AlgoDeploymentStatus {
	if in == nil {
		return nil
	}
	out := new(AlgoDeploymentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlgoInputModel) DeepCopyInto(out *AlgoInputModel) {
	*out = *in
	if in.ContentTypes != nil {
		in, out := &in.ContentTypes, &out.ContentTypes
		*out = make([]ContentTypeModel, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlgoInputModel.
func (in *AlgoInputModel) DeepCopy() *AlgoInputModel {
	if in == nil {
		return nil
	}
	out := new(AlgoInputModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlgoOutputModel) DeepCopyInto(out *AlgoOutputModel) {
	*out = *in
	if in.ContentType != nil {
		in, out := &in.ContentType, &out.ContentType
		*out = new(ContentTypeModel)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlgoOutputModel.
func (in *AlgoOutputModel) DeepCopy() *AlgoOutputModel {
	if in == nil {
		return nil
	}
	out := new(AlgoOutputModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlgoParamModel) DeepCopyInto(out *AlgoParamModel) {
	*out = *in
	if in.DataType != nil {
		in, out := &in.DataType, &out.DataType
		*out = new(DataTypeModel)
		**out = **in
	}
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make([]DataTypeOptionModel, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlgoParamModel.
func (in *AlgoParamModel) DeepCopy() *AlgoParamModel {
	if in == nil {
		return nil
	}
	out := new(AlgoParamModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlgoPodStatus) DeepCopyInto(out *AlgoPodStatus) {
	*out = *in
	if in.ContainerStatuses != nil {
		in, out := &in.ContainerStatuses, &out.ContainerStatuses
		*out = make([]v1.ContainerStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlgoPodStatus.
func (in *AlgoPodStatus) DeepCopy() *AlgoPodStatus {
	if in == nil {
		return nil
	}
	out := new(AlgoPodStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlgoRunnerConfig) DeepCopyInto(out *AlgoRunnerConfig) {
	*out = *in
	if in.AlgoParams != nil {
		in, out := &in.AlgoParams, &out.AlgoParams
		*out = make([]AlgoParamModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Inputs != nil {
		in, out := &in.Inputs, &out.Inputs
		*out = make([]AlgoInputModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Outputs != nil {
		in, out := &in.Outputs, &out.Outputs
		*out = make([]AlgoOutputModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Pipes != nil {
		in, out := &in.Pipes, &out.Pipes
		*out = make([]PipeModel, len(*in))
		copy(*out, *in)
	}
	if in.TopicConfigs != nil {
		in, out := &in.TopicConfigs, &out.TopicConfigs
		*out = make([]TopicConfigModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlgoRunnerConfig.
func (in *AlgoRunnerConfig) DeepCopy() *AlgoRunnerConfig {
	if in == nil {
		return nil
	}
	out := new(AlgoRunnerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContentTypeModel) DeepCopyInto(out *ContentTypeModel) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContentTypeModel.
func (in *ContentTypeModel) DeepCopy() *ContentTypeModel {
	if in == nil {
		return nil
	}
	out := new(ContentTypeModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataConnectorConfig) DeepCopyInto(out *DataConnectorConfig) {
	*out = *in
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make([]DataConnectorOptionModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataConnectorConfig.
func (in *DataConnectorConfig) DeepCopy() *DataConnectorConfig {
	if in == nil {
		return nil
	}
	out := new(DataConnectorConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataConnectorOptionModel) DeepCopyInto(out *DataConnectorOptionModel) {
	*out = *in
	if in.DataType != nil {
		in, out := &in.DataType, &out.DataType
		*out = new(DataTypeModel)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataConnectorOptionModel.
func (in *DataConnectorOptionModel) DeepCopy() *DataConnectorOptionModel {
	if in == nil {
		return nil
	}
	out := new(DataConnectorOptionModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataTypeModel) DeepCopyInto(out *DataTypeModel) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataTypeModel.
func (in *DataTypeModel) DeepCopy() *DataTypeModel {
	if in == nil {
		return nil
	}
	out := new(DataTypeModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DataTypeOptionModel) DeepCopyInto(out *DataTypeOptionModel) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataTypeOptionModel.
func (in *DataTypeOptionModel) DeepCopy() *DataTypeOptionModel {
	if in == nil {
		return nil
	}
	out := new(DataTypeOptionModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeepCopyTime.
func (in *DeepCopyTime) DeepCopy() *DeepCopyTime {
	if in == nil {
		return nil
	}
	out := new(DeepCopyTime)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentStatusMessage) DeepCopyInto(out *DeploymentStatusMessage) {
	*out = *in
	if in.AlgoDeploymentStatus != nil {
		in, out := &in.AlgoDeploymentStatus, &out.AlgoDeploymentStatus
		*out = new(AlgoDeploymentStatus)
		**out = **in
	}
	if in.AlgoPodStatus != nil {
		in, out := &in.AlgoPodStatus, &out.AlgoPodStatus
		*out = new(AlgoPodStatus)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentStatusMessage.
func (in *DeploymentStatusMessage) DeepCopy() *DeploymentStatusMessage {
	if in == nil {
		return nil
	}
	out := new(DeploymentStatusMessage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EndpointConfig) DeepCopyInto(out *EndpointConfig) {
	*out = *in
	if in.Paths != nil {
		in, out := &in.Paths, &out.Paths
		*out = make([]EndpointPathModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Server != nil {
		in, out := &in.Server, &out.Server
		*out = new(EndpointServerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Producer != nil {
		in, out := &in.Producer, &out.Producer
		*out = new(EndpointProducerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Kafka != nil {
		in, out := &in.Kafka, &out.Kafka
		*out = new(EndpointKafkaConfig)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EndpointConfig.
func (in *EndpointConfig) DeepCopy() *EndpointConfig {
	if in == nil {
		return nil
	}
	out := new(EndpointConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EndpointKafkaConfig) DeepCopyInto(out *EndpointKafkaConfig) {
	*out = *in
	if in.Brokers != nil {
		in, out := &in.Brokers, &out.Brokers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EndpointKafkaConfig.
func (in *EndpointKafkaConfig) DeepCopy() *EndpointKafkaConfig {
	if in == nil {
		return nil
	}
	out := new(EndpointKafkaConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EndpointPathModel) DeepCopyInto(out *EndpointPathModel) {
	*out = *in
	if in.ContentType != nil {
		in, out := &in.ContentType, &out.ContentType
		*out = new(ContentTypeModel)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EndpointPathModel.
func (in *EndpointPathModel) DeepCopy() *EndpointPathModel {
	if in == nil {
		return nil
	}
	out := new(EndpointPathModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EndpointProducerConfig) DeepCopyInto(out *EndpointProducerConfig) {
	*out = *in
	if in.Cb != nil {
		in, out := &in.Cb, &out.Cb
		*out = new(ProducerCircuitBreaker)
		**out = **in
	}
	if in.Resend != nil {
		in, out := &in.Resend, &out.Resend
		*out = new(ProducerResend)
		**out = **in
	}
	if in.Wal != nil {
		in, out := &in.Wal, &out.Wal
		*out = new(ProducerWal)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EndpointProducerConfig.
func (in *EndpointProducerConfig) DeepCopy() *EndpointProducerConfig {
	if in == nil {
		return nil
	}
	out := new(EndpointProducerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EndpointServerConfig) DeepCopyInto(out *EndpointServerConfig) {
	*out = *in
	if in.Http != nil {
		in, out := &in.Http, &out.Http
		*out = new(EndpointServerListen)
		**out = **in
	}
	if in.Grpc != nil {
		in, out := &in.Grpc, &out.Grpc
		*out = new(EndpointServerListen)
		**out = **in
	}
	if in.Monitoring != nil {
		in, out := &in.Monitoring, &out.Monitoring
		*out = new(EndpointServerListen)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EndpointServerConfig.
func (in *EndpointServerConfig) DeepCopy() *EndpointServerConfig {
	if in == nil {
		return nil
	}
	out := new(EndpointServerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EndpointServerListen) DeepCopyInto(out *EndpointServerListen) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EndpointServerListen.
func (in *EndpointServerListen) DeepCopy() *EndpointServerListen {
	if in == nil {
		return nil
	}
	out := new(EndpointServerListen)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HookConfig) DeepCopyInto(out *HookConfig) {
	*out = *in
	if in.WebHooks != nil {
		in, out := &in.WebHooks, &out.WebHooks
		*out = make([]WebHookModel, len(*in))
		copy(*out, *in)
	}
	if in.Pipes != nil {
		in, out := &in.Pipes, &out.Pipes
		*out = make([]PipeModel, len(*in))
		copy(*out, *in)
	}
	if in.TopicConfigs != nil {
		in, out := &in.TopicConfigs, &out.TopicConfigs
		*out = make([]TopicConfigModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HookConfig.
func (in *HookConfig) DeepCopy() *HookConfig {
	if in == nil {
		return nil
	}
	out := new(HookConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NotifMessage) DeepCopyInto(out *NotifMessage) {
	*out = *in
	in.MessageTimestamp = out.MessageTimestamp
	if in.DeploymentStatusMessage != nil {
		in, out := &in.DeploymentStatusMessage, &out.DeploymentStatusMessage
		*out = new(DeploymentStatusMessage)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NotifMessage.
func (in *NotifMessage) DeepCopy() *NotifMessage {
	if in == nil {
		return nil
	}
	out := new(NotifMessage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipeModel) DeepCopyInto(out *PipeModel) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipeModel.
func (in *PipeModel) DeepCopy() *PipeModel {
	if in == nil {
		return nil
	}
	out := new(PipeModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineDeployment) DeepCopyInto(out *PipelineDeployment) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineDeployment.
func (in *PipelineDeployment) DeepCopy() *PipelineDeployment {
	if in == nil {
		return nil
	}
	out := new(PipelineDeployment)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineDeployment) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineDeploymentList) DeepCopyInto(out *PipelineDeploymentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PipelineDeployment, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineDeploymentList.
func (in *PipelineDeploymentList) DeepCopy() *PipelineDeploymentList {
	if in == nil {
		return nil
	}
	out := new(PipelineDeploymentList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineDeploymentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineDeploymentSpec) DeepCopyInto(out *PipelineDeploymentSpec) {
	*out = *in
	in.PipelineSpec.DeepCopyInto(&out.PipelineSpec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineDeploymentSpec.
func (in *PipelineDeploymentSpec) DeepCopy() *PipelineDeploymentSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineDeploymentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineDeploymentStatus) DeepCopyInto(out *PipelineDeploymentStatus) {
	*out = *in
	if in.AlgoDeploymentStatuses != nil {
		in, out := &in.AlgoDeploymentStatuses, &out.AlgoDeploymentStatuses
		*out = make([]AlgoDeploymentStatus, len(*in))
		copy(*out, *in)
	}
	if in.AlgoPodStatuses != nil {
		in, out := &in.AlgoPodStatuses, &out.AlgoPodStatuses
		*out = make([]AlgoPodStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineDeploymentStatus.
func (in *PipelineDeploymentStatus) DeepCopy() *PipelineDeploymentStatus {
	if in == nil {
		return nil
	}
	out := new(PipelineDeploymentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineSpec) DeepCopyInto(out *PipelineSpec) {
	*out = *in
	if in.EndpointConfig != nil {
		in, out := &in.EndpointConfig, &out.EndpointConfig
		*out = new(EndpointConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.AlgoConfigs != nil {
		in, out := &in.AlgoConfigs, &out.AlgoConfigs
		*out = make([]AlgoConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.DataConnectorConfigs != nil {
		in, out := &in.DataConnectorConfigs, &out.DataConnectorConfigs
		*out = make([]DataConnectorConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.HookConfig != nil {
		in, out := &in.HookConfig, &out.HookConfig
		*out = new(HookConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.TopicConfigs != nil {
		in, out := &in.TopicConfigs, &out.TopicConfigs
		*out = make([]TopicConfigModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Pipes != nil {
		in, out := &in.Pipes, &out.Pipes
		*out = make([]PipeModel, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineSpec.
func (in *PipelineSpec) DeepCopy() *PipelineSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProducerCircuitBreaker) DeepCopyInto(out *ProducerCircuitBreaker) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProducerCircuitBreaker.
func (in *ProducerCircuitBreaker) DeepCopy() *ProducerCircuitBreaker {
	if in == nil {
		return nil
	}
	out := new(ProducerCircuitBreaker)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProducerResend) DeepCopyInto(out *ProducerResend) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProducerResend.
func (in *ProducerResend) DeepCopy() *ProducerResend {
	if in == nil {
		return nil
	}
	out := new(ProducerResend)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProducerWal) DeepCopyInto(out *ProducerWal) {
	*out = *in
	if in.AlwaysTopics != nil {
		in, out := &in.AlwaysTopics, &out.AlwaysTopics
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DisableTopics != nil {
		in, out := &in.DisableTopics, &out.DisableTopics
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProducerWal.
func (in *ProducerWal) DeepCopy() *ProducerWal {
	if in == nil {
		return nil
	}
	out := new(ProducerWal)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TopicConfigModel) DeepCopyInto(out *TopicConfigModel) {
	*out = *in
	if in.TopicParams != nil {
		in, out := &in.TopicParams, &out.TopicParams
		*out = make([]TopicParamModel, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopicConfigModel.
func (in *TopicConfigModel) DeepCopy() *TopicConfigModel {
	if in == nil {
		return nil
	}
	out := new(TopicConfigModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TopicParamModel) DeepCopyInto(out *TopicParamModel) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopicParamModel.
func (in *TopicParamModel) DeepCopy() *TopicParamModel {
	if in == nil {
		return nil
	}
	out := new(TopicParamModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WebHookModel) DeepCopyInto(out *WebHookModel) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WebHookModel.
func (in *WebHookModel) DeepCopy() *WebHookModel {
	if in == nil {
		return nil
	}
	out := new(WebHookModel)
	in.DeepCopyInto(out)
	return out
}

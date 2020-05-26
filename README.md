## Introduction

The Algo.Run Pipeline Operator facilitates versioned, portable and easily deployable automation pipelines on any Kubernetes cluster. Your processing nodes (Algos) can be created with any script, executable or server that can be containerized. 
This provides some powerful capabilities:

* **Integration** - Easy integration with existing code. No need to throw away existing work or rewrite for yet another framework.
* **Language Agnostic** - Golang, Python, C++, R, Java, C#, bash, Rust... you name it and it can be a be imported as a processing node.

A pipeline deployment can be created through a simple yet powerful declarative API implemented as Kubernetes Custom Resources. 

Check out the [documentation](https://algohub.com/lc/docs) for more info.

## Features
* **Declarative API** - Create reusable pipelines using purely YAML definitions
* **Processing Nodes** - Use any HTTP, gRPC microservice or even executables as processing algos
* **Data Connectors** - Provisions Kafka Connect data connectors
* **Endpoint Management** - Deploys HTTP and gRPC endpoints to push data into the pipeline from external systems
* **Event Hooks** - Subscribe to pipeline events with webhooks to receive output data
* **Security** - Automatic provisioning of TLS for Kafka and Endpoints. mTLS, sasl scram, plain Kafka authentication.
* **Retry Strategies** - Built-in Kafka retry and dead-letter-queue strategies.
* **Monitoring** - Prometheus metrics for all components and prebuilt grafana dashboards

## Requirements

Algo.Run Pipeline Operator runs on Kubernetes and is compatible with all major distributions. We have tested on:
- [Minikube](https://github.com/kubernetes/minikube)
- [Docker Desktop](https://www.docker.com/products/docker-desktop)
- [MicroK8s](https://microk8s.io/)
- [AWS EKS](https://aws.amazon.com/eks/)
- [Google Cloud GKE](https://cloud.google.com/kubernetes-engine)
- [Azure AKS](https://azure.microsoft.com/en-us/services/kubernetes-service/)

It should also run fine on all other local or cloud k8s distributions.


The data plane for algo.run pipelines is built on Kafka and thus a Kafka cluster is required. Currently, in order to automatically provision Kafka Connect data connectors, Kafka Topics, mTLS and encryption, the [Strimzi Kafka Operator](https://strimzi.io/) must be installed and configured in the Kubernetes cluster. 
To install the strimzi kafka operator:
- [New Kafka cluster](https://strimzi.io/docs/quickstart/latest/#proc-install-product-str)

For a development Kafka cluster ready for algo.run:
```bash
$ helm repo add strimzi http://strimzi.io/charts/
$ kubectl create namespace kafka
$ helm install kafka-operator strimzi/strimzi-kafka-operator --set watchNamespaces="{algorun}" --namespace kafka
```

The pipeline operator also utilizes [Ambassador](https://www.getambassador.io/) to create dynamic ingress mappings into pipeline deployment endpoints.

For a development Ambassador deployment ready for algo.run:
```bash
$ helm repo add datawire https://getambassador.io
$ kubectl create namespace ambassador
$ helm install ambassador datawire/ambassador --set enableAES="false" --namespace ambassador
```

Storing data in S3 compatible storage is also supported (not required), if embedding the output data in the kafka stream is not desired.
We recommend [Minio](https://min.io/) for storage within the kubernetes cluster, otherwise any external S3 bucket can be used.

## Installation

The easiest way to deploy the pipeline operator into your k8s environment is helm. 
For detailed information, check out the [helm chart](https://github.com/algohubhq/helm-charts/tree/master/pipeline-operator).

#### Install helm
https://github.com/helm/helm#install

#### Add the algohub helm chart repo
```bash
$ helm repo add algohub https://charts.algohub.com
```

#### Creating a separate namespace for algo.run resources is recommended
```bash
$ kubectl create namespace algorun
```

#### Install the pipeline operator into the namespace
```bash
$ helm install algorun-pipeline-operator algohub/algorun-pipeline-operator --namespace algorun
```

For detailed information on the available helm values, check out the [helm chart readme](https://github.com/algohubhq/helm-charts/tree/master/pipeline-operator).
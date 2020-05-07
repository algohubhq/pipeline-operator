module pipeline-operator

go 1.13

require (
	github.com/go-kafka/connect v0.9.0 // indirect
	github.com/go-test/deep v1.0.1
	github.com/minio/minio-go/v6 v6.0.49
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/prometheus/client_golang v1.5.1
	github.com/spf13/pflag v1.0.5
	github.com/xorcare/pointer v1.1.0 // indirect
	golang.org/x/net v0.0.0-20200320181208-1c781a10960a // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.5.2
	sigs.k8s.io/testing_frameworks v0.1.2 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
)

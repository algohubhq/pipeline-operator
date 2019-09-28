### Setup Service Account
kubectl create -f deploy/service_account.yaml
### Setup RBAC
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
### Setup the CRD
kubectl create -f deploy/crds/algo_v1alpha1_pipelinedeployment_crd.yaml
### Deploy the app-operator
kubectl create -f deploy/operator.yaml
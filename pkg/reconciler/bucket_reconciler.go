package reconciler

import (
	"context"
	"fmt"
	"net/url"
	"os"
	algov1beta1 "pipeline-operator/pkg/apis/algorun/v1beta1"
	utils "pipeline-operator/pkg/utilities"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/minio/minio-go/v6"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewBucketReconciler returns a new BucketReconciler
func NewBucketReconciler(pipelineDeployment *algov1beta1.PipelineDeployment,
	request *reconcile.Request,
	manager manager.Manager) BucketReconciler {
	return BucketReconciler{
		pipelineDeployment: pipelineDeployment,
		request:            request,
		manager:            manager,
	}
}

// BucketReconciler reconciles the S3 bucket for a deployment
type BucketReconciler struct {
	pipelineDeployment *algov1beta1.PipelineDeployment
	request            *reconcile.Request
	manager            manager.Manager
}

// Reconcile executes the Storage Bucket reconciliation process
func (bucketReconciler *BucketReconciler) Reconcile() error {

	kubeUtil := utils.NewKubeUtil(bucketReconciler.manager, bucketReconciler.request)
	storageSecretName, err := kubeUtil.GetStorageSecretName(&bucketReconciler.pipelineDeployment.Spec)

	if storageSecretName != "" && err == nil {

		// Get the MC config secret
		storageSecret := &corev1.Secret{}
		err := bucketReconciler.manager.GetClient().Get(
			context.TODO(),
			types.NamespacedName{
				Name:      storageSecretName,
				Namespace: bucketReconciler.pipelineDeployment.Spec.DeploymentNamespace,
			},
			storageSecret)

		if err != nil {
			return err
		}
		// Parse the secret
		endpoint, accessKey, secret, err := parseEnvURLStr(string(storageSecret.Data["connection-string"]))
		if err != nil {
			return err
		}

		// Create the bucket
		minioClient, err := minio.New(endpoint.Host, accessKey, secret, endpoint.Scheme == "https")
		if err != nil {
			return err
		}

		// default bucket name
		bucketName := fmt.Sprintf("%s.%s",
			strings.ToLower(bucketReconciler.pipelineDeployment.Spec.DeploymentOwner),
			strings.ToLower(bucketReconciler.pipelineDeployment.Spec.DeploymentName))

		// Use env var if set
		bucketNameEnv := os.Getenv("STORAGE_BUCKET_NAME")
		if bucketNameEnv != "" {
			bucketName = strings.Replace(bucketNameEnv, "{deploymentowner}", bucketReconciler.pipelineDeployment.Spec.DeploymentOwner, -1)
			bucketName = strings.Replace(bucketName, "{deploymentname}", bucketReconciler.pipelineDeployment.Spec.DeploymentName, -1)
			bucketName = strings.ToLower(bucketName)
		}

		// default region
		regionName := "us-east-1"
		// Use env var if set
		regionNameEnv := os.Getenv("STORAGE_REGION")
		if regionNameEnv != "" {
			regionName = regionNameEnv
		}

		exists, err := minioClient.BucketExists(bucketName)
		if err != nil {
			return err
		}

		if !exists {
			err = minioClient.MakeBucket(bucketName, regionName)
			if err != nil {
				return err
			}
		}

	} else {
		log.Error(err, "Storage Connection String secret doesn't exist. Unable to reconcile storage bucket.")
	}

	return nil

}

// parse url usually obtained from env.
func parseEnvURL(envURL string) (*url.URL, string, string, error) {
	u, e := url.Parse(envURL)
	if e != nil {
		return nil, "", "", fmt.Errorf("S3 Endpoint url invalid [%s]", envURL)
	}

	var accessKey, secretKey string
	// Check if username:password is provided in URL, with no
	// access keys or secret we proceed and perform anonymous
	// requests.
	if u.User != nil {
		accessKey = u.User.Username()
		secretKey, _ = u.User.Password()
	}

	// Look for if URL has invalid values and return error.
	if !((u.Scheme == "http" || u.Scheme == "https") &&
		(u.Path == "/" || u.Path == "") && u.Opaque == "" &&
		!u.ForceQuery && u.RawQuery == "" && u.Fragment == "") {
		return nil, "", "", fmt.Errorf("S3 Endpoint url invalid [%s]", u.String())
	}

	// Now that we have validated the URL to be in expected style.
	u.User = nil

	return u, accessKey, secretKey, nil
}

// parse url usually obtained from env.
func parseEnvURLStr(envURL string) (*url.URL, string, string, error) {
	var envURLStr string
	u, accessKey, secretKey, err := parseEnvURL(envURL)
	if err != nil {
		// url parsing can fail when accessKey/secretKey contains non url encoded values
		// such as #. Strip accessKey/secretKey from envURL and parse again.
		re := regexp.MustCompile("^(https?://)(.*?):(.*?)@(.*?)$")
		res := re.FindAllStringSubmatch(envURL, -1)
		// regex will return full match, scheme, accessKey, secretKey and endpoint:port as
		// captured groups.
		if res == nil || len(res[0]) != 5 {
			return nil, "", "", err
		}
		for k, v := range res[0] {
			if k == 2 {
				accessKey = v
			}
			if k == 3 {
				secretKey = v
			}
			if k == 1 || k == 4 {
				envURLStr = fmt.Sprintf("%s%s", envURLStr, v)
			}
		}
		u, _, _, err = parseEnvURL(envURLStr)
		if err != nil {
			return nil, "", "", err
		}
	}
	// Check if username:password is provided in URL, with no
	// access keys or secret we proceed and perform anonymous
	// requests.
	if u.User != nil {
		accessKey = u.User.Username()
		secretKey, _ = u.User.Password()
	}
	return u, accessKey, secretKey, nil
}

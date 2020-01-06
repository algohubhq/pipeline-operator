package reconciler

import (
	"context"
	"fmt"
	"net/url"
	algov1alpha1 "pipeline-operator/pkg/apis/algo/v1alpha1"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/minio/minio-go/v6"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewBucketReconciler returns a new BucketReconciler
func NewBucketReconciler(pipelineDeployment *algov1alpha1.PipelineDeployment,
	request *reconcile.Request,
	client client.Client) BucketReconciler {
	return BucketReconciler{
		pipelineDeployment: pipelineDeployment,
		request:            request,
		client:             client,
	}
}

// BucketReconciler reconciles the S3 bucket for a deployment
type BucketReconciler struct {
	pipelineDeployment *algov1alpha1.PipelineDeployment
	request            *reconcile.Request
	client             client.Client
}

func (bucketReconciler *BucketReconciler) Reconcile() error {

	// Get the MC config secret
	storageSecret := &corev1.Secret{}
	err := bucketReconciler.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "storage-endpoint",
			Namespace: bucketReconciler.request.NamespacedName.Namespace,
		},
		storageSecret)

	if err != nil {
		return err
	}
	// Parse the secret
	endpoint, accessKey, secret, err := parseEnvURLStr(string(storageSecret.Data["mc"]))
	if err != nil {
		return err
	}

	// Create the bucket
	minioClient, err := minio.New(endpoint.Host, accessKey, secret, endpoint.Scheme == "https")
	if err != nil {
		return err
	}

	bucketName := fmt.Sprintf("%s.%s",
		strings.ToLower(bucketReconciler.pipelineDeployment.Spec.PipelineSpec.DeploymentOwnerUserName),
		strings.ToLower(bucketReconciler.pipelineDeployment.Spec.PipelineSpec.DeploymentName))

	exists, err := minioClient.BucketExists(bucketName)
	if err != nil {
		return err
	}

	if !exists {
		err = minioClient.MakeBucket(bucketName, "us-east-1")
		if err != nil {
			return err
		}
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

package k8s

import (
	"bytes"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

func UpdateServiceAccount(k8sClient *kubernetes.Clientset, serviceAccountYaml string) error {
	serviceAccount := v1.ServiceAccount{}

	decoder := yamlutil.NewYAMLToJSONDecoder(bytes.NewReader([]byte(serviceAccountYaml)))
	if err := decoder.Decode(&serviceAccount); err != nil {
		return err
	}

	if _, err := k8sClient.CoreV1().ServiceAccounts(metav1.NamespaceSystem).Create(&serviceAccount); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if _, err := k8sClient.CoreV1().ServiceAccounts(metav1.NamespaceSystem).Update(&serviceAccount); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

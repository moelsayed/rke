package k8s

import (
	"bytes"

	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

func ApplySystemNodeClusterRoleBinding(kubeConfigPath string) error {
	systemNodeClusterRoleBinding := `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: system:node
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:node
subjects:
- kind: Group
  name: system:nodes
  apiGroup: rbac.authorization.k8s.io`

	k8sClient, err := NewClient(kubeConfigPath)
	if err != nil {
		return err
	}
	return UpdateClusterRoleBinding(k8sClient, systemNodeClusterRoleBinding)
}

func UpdateClusterRoleBinding(k8sClient *kubernetes.Clientset, clusterRoleBindingYaml string) error {
	clusterRoleBinding := rbacv1beta1.ClusterRoleBinding{}
	decoder := yamlutil.NewYAMLToJSONDecoder(bytes.NewReader([]byte(clusterRoleBindingYaml)))
	if err := decoder.Decode(&clusterRoleBinding); err != nil {
		return err
	}
	if _, err := k8sClient.RbacV1beta1().ClusterRoleBindings().Create(&clusterRoleBinding); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if _, err := k8sClient.RbacV1beta1().ClusterRoleBindings().Update(&clusterRoleBinding); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

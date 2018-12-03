package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	BackupConditionCreated    condition.Cond = "Created"
	BackupConditionCompeleted condition.Cond = "Compeleted"
)

type BackupBackend struct {
	// backend name
	Name string `yaml:"name" json:"name,omitempty" norman:"default=s3"`
	// s3 backend
	S3BackupBackend *S3BackupBackend `yaml:",omitempty" json:"s3BackupBackend"`
}

type S3BackupBackend struct {
	// Access key ID
	AccessKeyID string `yaml:"access_key_id" json:"accessKeyId,omitempty"`
	// Secret access key
	SecretAccesssKey string `yaml:"secret_access_key" json:"secretAccessKey,omitempty"`
	// name of the bucket to use for backup
	BucketName string `yaml:"bucket_name" json:"bucketName,omitempty"`
	// AWS Region, AWS spcific
	Region string `yaml:"region" json:"region,omitempty"`
	// Endpoint is used if this is not an AWS API
	Endpoint string `yaml:"endpoint" json:"endpoint"`
}

type EtcdBackup struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ClusterID string `json:"clusterId,omitempty" norman:"required,type=reference[cluster]"`
	// s3 backup type
	S3EtcdBackup *S3EtcdBackup `yaml:",omitempty" json:"s3EtcdBackup,omitempty"`
	// backup status
	Status EtcdBackupStatus `yaml:"status" json:"status,omitempty"`
}

type S3EtcdBackup struct {
	// actual file name on the backend
	FileName string `yaml:"file_name" json:"fileName,omitempty"`
	// Backend configuration used for this backup
	S3BackupBackend S3BackupBackend `yaml:",omitempty" json:"s3BackupBackend"`
}

type EtcdBackupStatus struct {
	Conditions []EtcdBackupCondition `json:"conditions"`
}

type EtcdBackupCondition struct {
	// Type of condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

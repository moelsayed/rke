package state

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/client-go/util/cert"
)

type RKEState struct {
	DesiredState v3.RKEPlan `json:"desiredState"`
	CurrentState v3.RKEPlan `json:"currentState"`
}

func GetDesiredState(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, hostsInfoMap map[string]types.Info) (v3.RKEPlan, error) {
	// Generate a Plan from RKEConfig
	DesiredState, err := cluster.GeneratePlan(ctx, rkeConfig, hostsInfoMap)
	if err != nil {
		return DesiredState, fmt.Errorf("Failed to generate plan for desired state: %v", err)
	}
	// Get the certificate Bundle
	certBundle, err := pki.GenerateRKECerts(ctx, *rkeConfig, "", "")
	if err != nil {
		return DesiredState, fmt.Errorf("Failed to generate certificate bundle: %v", err)
	}
	desiredStateCertificateBundle := make(map[string]v3.CertificatePKI)
	// Convert rke certs to v3.certs
	for name, certPKI := range certBundle {
		certificatePEM := string(cert.EncodeCertPEM(certPKI.Certificate))
		keyPEM := string(cert.EncodePrivateKeyPEM(certPKI.Key))
		DesiredState.CertificatesBundle[name] = v3.CertificatePKI{
			Name:        certPKI.Name,
			Config:      certPKI.Config,
			Certificate: certificatePEM,
			Key:         keyPEM,
		}
	}
	DesiredState.CertificatesBundle = desiredStateCertificateBundle
	return DesiredState, nil
}

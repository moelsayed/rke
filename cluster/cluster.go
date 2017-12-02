package cluster

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/cluster.cattle.io/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
)

type Cluster struct {
	v1.RancherKubernetesEngineConfig `yaml:",inline"`
	ConfigPath                       string `yaml:"config_path"`
	LocalKubeConfigPath              string
	EtcdHosts                        []hosts.Host
	WorkerHosts                      []hosts.Host
	ControlPlaneHosts                []hosts.Host
	KubeClient                       *kubernetes.Clientset
	KubernetesServiceIP              net.IP
	Certificates                     map[string]pki.CertificatePKI
	ClusterDomain                    string
	ClusterCIDR                      string
	ClusterDNSServer                 string
}

const (
	X509AuthenticationProvider = "x509"
	StateConfigMapName         = "cluster-state"
	UpdateStateTimeout         = 30
	GetStateTimeout            = 30
	KubernetesClientTimeOut    = 30
)

func (c *Cluster) DeployClusterPlanes() error {
	// Deploy Kubernetes Planes
	err := services.RunEtcdPlane(c.EtcdHosts, c.Services.Etcd)
	if err != nil {
		return fmt.Errorf("[etcd] Failed to bring up Etcd Plane: %v", err)
	}
	err = services.RunControlPlane(c.ControlPlaneHosts, c.EtcdHosts, c.Services)
	if err != nil {
		return fmt.Errorf("[controlPlane] Failed to bring up Control Plane: %v", err)
	}
	err = services.RunWorkerPlane(c.ControlPlaneHosts, c.WorkerHosts, c.Services)
	if err != nil {
		return fmt.Errorf("[workerPlane] Failed to bring up Worker Plane: %v", err)
	}
	return nil
}

func ParseConfig(clusterFile string) (*Cluster, error) {
	logrus.Debugf("Parsing cluster file [%v]", clusterFile)
	var err error
	c, err := parseClusterFile(clusterFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse the cluster file: %v", err)
	}
	err = c.InvertIndexHosts()
	if err != nil {
		return nil, fmt.Errorf("Failed to classify hosts from config file: %v", err)
	}

	err = c.ValidateCluster()
	if err != nil {
		return nil, fmt.Errorf("Failed to validate cluster: %v", err)
	}

	c.KubernetesServiceIP, err = services.GetKubernetesServiceIP(c.Services.KubeAPI.ServiceClusterIPRange)
	if err != nil {
		return nil, fmt.Errorf("Failed to get Kubernetes Service IP: %v", err)
	}
	c.ClusterDomain = c.Services.Kubelet.ClusterDomain
	c.ClusterCIDR = c.Services.KubeController.ClusterCIDR
	c.ClusterDNSServer = c.Services.Kubelet.ClusterDNSServer
	if len(c.ConfigPath) == 0 {
		c.ConfigPath = DefaultClusterConfig
	}
	c.LocalKubeConfigPath = GetLocalKubeConfig(c.ConfigPath)
	return c, nil
}

func parseClusterFile(clusterFile string) (*Cluster, error) {
	// parse hosts
	var kubeCluster Cluster
	err := yaml.Unmarshal([]byte(clusterFile), &kubeCluster)
	if err != nil {
		return nil, err
	}
	// Setting cluster Defaults
	kubeCluster.setClusterDefaults()

	return &kubeCluster, nil
}

func (c *Cluster) setClusterDefaults() {
	for i, host := range c.Nodes {
		if len(host.InternalAddress) == 0 {
			c.Nodes[i].InternalAddress = c.Nodes[i].Address
		}
		if len(host.HostnameOverride) == 0 {
			// This is a temporary modification
			c.Nodes[i].HostnameOverride = c.Nodes[i].Address
		}
	}
	setDefaultIfEmpty(&c.Services.KubeAPI.ServiceClusterIPRange, DefaultServiceClusterIPRange)
	setDefaultIfEmpty(&c.Services.KubeController.ServiceClusterIPRange, DefaultServiceClusterIPRange)
	setDefaultIfEmpty(&c.Services.KubeController.ClusterCIDR, DefaultClusterCIDR)
	setDefaultIfEmpty(&c.Services.Kubelet.ClusterDNSServer, DefaultClusterDNSService)
	setDefaultIfEmpty(&c.Services.Kubelet.ClusterDomain, DefaultClusterDomain)
	setDefaultIfEmpty(&c.Services.Kubelet.InfraContainerImage, DefaultInfraContainerImage)
	setDefaultIfEmpty(&c.Authentication.Strategy, DefaultAuthStrategy)

	c.setClusterNetworkDefaults()
	c.setClusterImageDefaults()
}

func (c *Cluster) setClusterImageDefaults() {
	if c.RKEImages == nil {
		// don't break if the user didn't define rke_images
		c.RKEImages = make(map[string]string)
	}
	setDefaultIfEmptyMapValue(c.RKEImages, "alpine", DefaultAplineImage)
	setDefaultIfEmptyMapValue(c.RKEImages, "nginx_proxy", DefaultNginxProxyImage)
	setDefaultIfEmptyMapValue(c.RKEImages, "cert_downloader", DefaultCertDownloaderImage)
	setDefaultIfEmptyMapValue(c.RKEImages, "kubedns_image", DefaultKubeDNSImage)
	setDefaultIfEmptyMapValue(c.RKEImages, "dnsmasq_image", DefaultDNSMasqImage)
	setDefaultIfEmptyMapValue(c.RKEImages, "kubedns_sidecar_image", DefaultKubeDNSSidecarImage)
	setDefaultIfEmptyMapValue(c.RKEImages, "kubedns_autoscaler_image", DefaultKubeDNSAutoScalerImage)
}

func setDefaultIfEmptyMapValue(configMap map[string]string, key string, value string) {
	if _, ok := configMap[key]; !ok {
		configMap[key] = value
	}
}
func setDefaultIfEmpty(varName *string, defaultValue string) {
	if len(*varName) == 0 {
		*varName = defaultValue
	}
}
func GetLocalKubeConfig(configPath string) string {
	baseDir := filepath.Dir(configPath)
	fileName := filepath.Base(configPath)
	baseDir += "/"
	return fmt.Sprintf("%s%s%s", baseDir, pki.KubeAdminConfigPrefix, fileName)
}

func ReconcileCluster(kubeCluster, currentCluster *Cluster) error {
	logrus.Infof("[reconcile] Reconciling cluster state")
	if currentCluster == nil {
		logrus.Infof("[reconcile] This is newly generated cluster")
		return nil
	}
	if err := rebuildLocalAdminConfig(kubeCluster); err != nil {
		return err
	}
	kubeClient, err := k8s.NewClient(kubeCluster.LocalKubeConfigPath)
	if err != nil {
		return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
	}
	key, _ := checkEncryptedKey(kubeCluster.SSHKeyPath)

	logrus.Infof("[reconcile] Check Control plane hosts to be deleted")
	cpToDelete := hosts.GetToDeleteHosts(currentCluster.ControlPlaneHosts, kubeCluster.ControlPlaneHosts)
	for _, toDeleteHost := range cpToDelete {
		if err := hosts.DeleteNode(&toDeleteHost, kubeClient); err != nil {
			return fmt.Errorf("Failed to delete controlplane node %s from cluster", toDeleteHost.Address)
		}
		// attempting to clean up the host
		if err := reconcileHostCleaner(toDeleteHost, key, false, currentCluster.RKEImages["alpine"]); err != nil {
			logrus.Warnf("[reconcile] Couldn't clean up controlplane node [%s]: %v", toDeleteHost.Address, err)
			continue
		}
	}

	logrus.Infof("[reconcile] Check worker hosts to be deleted")
	wpToDelete := hosts.GetToDeleteHosts(currentCluster.WorkerHosts, kubeCluster.WorkerHosts)
	for _, toDeleteHost := range wpToDelete {
		if err := hosts.DeleteNode(&toDeleteHost, kubeClient); err != nil {
			return fmt.Errorf("Failed to delete worker node %s from cluster", toDeleteHost.Address)
		}
		// attempting to clean up the host
		if err := reconcileHostCleaner(toDeleteHost, key, true, currentCluster.RKEImages["apline"]); err != nil {
			logrus.Warnf("[reconcile] Couldn't clean up worker node [%s]: %v", toDeleteHost.Address, err)
			continue
		}
	}

	// Rolling update on change for nginx Proxy
	cpChanged := hosts.IsHostListChanged(currentCluster.ControlPlaneHosts, kubeCluster.ControlPlaneHosts)
	if cpChanged {
		logrus.Infof("[reconcile] Rolling update nginx hosts with new list of control plane hosts")
		err = services.RollingUpdateNginxProxy(kubeCluster.ControlPlaneHosts, kubeCluster.WorkerHosts)
		if err != nil {
			return fmt.Errorf("Failed to rolling update Nginx hosts with new control plane hosts")
		}
	}
	logrus.Infof("[reconcile] Reconciled cluster state successfully")
	return nil
}

func reconcileHostCleaner(toDeleteHost hosts.Host, key ssh.Signer, worker bool, cleanerImage string) error {
	if err := toDeleteHost.TunnelUp(key); err != nil {
		return fmt.Errorf("Not able to reach the host: %v", err)
	}
	if err := services.RemoveControlPlane([]hosts.Host{toDeleteHost}); err != nil {
		return fmt.Errorf("Couldn't remove control plane: %v", err)
	}

	if err := services.RemoveWorkerPlane(nil, []hosts.Host{toDeleteHost}); err != nil {
		return fmt.Errorf("Couldn't remove worker plane: %v", err)
	}
	if err := toDeleteHost.CleanUp(cleanerImage); err != nil {
		return fmt.Errorf("Not able to clean the host: %v", err)
	}
	return nil
}

func rebuildLocalAdminConfig(kubeCluster *Cluster) error {
	logrus.Infof("[reconcile] Rebuilding and update local kube config")
	currentKubeConfig := kubeCluster.Certificates[pki.KubeAdminCommonName]
	caCrt := kubeCluster.Certificates[pki.CACertName].Certificate
	newConfig := pki.GetKubeConfigX509WithData(
		"https://"+kubeCluster.ControlPlaneHosts[0].Address+":6443",
		pki.KubeAdminCommonName,
		string(cert.EncodeCertPEM(caCrt)),
		string(cert.EncodeCertPEM(currentKubeConfig.Certificate)),
		string(cert.EncodePrivateKeyPEM(currentKubeConfig.Key)))
	err := pki.DeployAdminConfig(newConfig, kubeCluster.LocalKubeConfigPath)
	if err != nil {
		return fmt.Errorf("Failed to redeploy local admin config with new host")
	}
	currentKubeConfig.Config = newConfig
	kubeCluster.Certificates[pki.KubeAdminCommonName] = currentKubeConfig
	return nil
}

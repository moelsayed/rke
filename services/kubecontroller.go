package services

import (
	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
)

type KubeController struct {
	Version               string `yaml:"version"`
	Image                 string `yaml:"image"`
	ClusterCIDR           string `yaml:"cluster_cider"`
	ServiceClusterIPRange string `yaml:"service_cluster_ip_range"`
}

func runKubeController(host hosts.Host, kubeControllerService KubeController) error {
	imageCfg, hostCfg := buildKubeControllerConfig(host, kubeControllerService)
	err := docker.DoRunContainer(imageCfg, hostCfg, KubeControllerContainerName, &host, ControlRole)
	if err != nil {
		return err
	}
	return nil
}

func buildKubeControllerConfig(host hosts.Host, kubeControllerService KubeController) (*container.Config, *container.HostConfig) {
	imageCfg := &container.Config{
		Image: kubeControllerService.Image + ":" + kubeControllerService.Version,
		Cmd: []string{"/hyperkube",
			"controller-manager",
			"--address=0.0.0.0",
			"--cloud-provider=",
			"--master=http://" + host.IP + ":8080",
			"--enable-hostpath-provisioner=false",
			"--node-monitor-grace-period=40s",
			"--pod-eviction-timeout=5m0s",
			"--v=2",
			"--allocate-node-cidrs=true",
			"--cluster-cidr=" + kubeControllerService.ClusterCIDR,
			"--service-cluster-ip-range=" + kubeControllerService.ServiceClusterIPRange},
	}
	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "always"},
	}
	return imageCfg, hostCfg
}

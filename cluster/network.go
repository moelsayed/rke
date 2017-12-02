package cluster

import (
	"fmt"

	"github.com/rancher/rke/network"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/sirupsen/logrus"
)

const (
	NetworkPluginResourceName = "rke-network-plugin"
	FlannelNetworkPlugin      = "flannel"
	CalicoNetworkPlugin       = "calico"
	CanalNetworkPlugin        = "canal"
)

func (c *Cluster) DeployNetworkPlugin() error {
	logrus.Infof("[network] Setting up network plugin: %s", c.Network.Plugin)
	switch c.Network.Plugin {
	case FlannelNetworkPlugin:
		return c.doFlannelDeploy()
	case CalicoNetworkPlugin:
		return c.doCalicoDeploy()
	case CanalNetworkPlugin:
		return c.doCanalDeploy()
	default:
		return fmt.Errorf("[network] Unsupported network plugin: %s", c.Network.Plugin)
	}
}

func (c *Cluster) doFlannelDeploy() error {
	pluginYaml := network.GetFlannelManifest(c.ClusterCIDR, c.Network.Options["flannel_image"], c.Network.Options["flannel_cni_image"])
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doCalicoDeploy() error {
	calicoConfig := make(map[string]string)
	calicoConfig["etcdEndpoints"] = services.GetEtcdConnString(c.EtcdHosts)
	calicoConfig["apiRoot"] = "https://127.0.0.1:6443"
	calicoConfig["clientCrt"] = pki.KubeNodeCertPath
	calicoConfig["clientKey"] = pki.KubeNodeKeyPath
	calicoConfig["clientCA"] = pki.CACertPath
	calicoConfig["kubeCfg"] = pki.KubeNodeConfigPath
	calicoConfig["clusterCIDR"] = c.ClusterCIDR
	calicoConfig["cni_image"] = c.Network.Options["calico_cni_image"]
	calicoConfig["node_image"] = c.Network.Options["calico_node_image"]
	calicoConfig["controllers_image"] = c.Network.Options["calico_controllers_image"]
	pluginYaml := network.GetCalicoManifest(calicoConfig)
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) doCanalDeploy() error {
	canalConfig := make(map[string]string)
	canalConfig["clientCrt"] = pki.KubeNodeCertPath
	canalConfig["clientKey"] = pki.KubeNodeKeyPath
	canalConfig["clientCA"] = pki.CACertPath
	canalConfig["kubeCfg"] = pki.KubeNodeConfigPath
	canalConfig["clusterCIDR"] = c.ClusterCIDR
	canalConfig["node_image"] = c.Network.Options["canal_node_image"]
	canalConfig["cni_image"] = c.Network.Options["canal_cni_image"]
	canalConfig["flannel_image"] = c.Network.Options["canal_flannel_image"]
	pluginYaml := network.GetCanalManifest(canalConfig)
	return c.doAddonDeploy(pluginYaml, NetworkPluginResourceName)
}

func (c *Cluster) setClusterNetworkDefaults() {
	setDefaultIfEmpty(&c.Network.Plugin, DefaultNetworkPlugin)

	if c.Network.Options == nil {
		// don't break if the user didn't define options
		c.Network.Options = make(map[string]string)
	}
	switch {
	case c.Network.Plugin == FlannelNetworkPlugin:
		setDefaultIfEmptyMapValue(c.Network.Options, "flannel_image", DefaultFlannelImage)
		setDefaultIfEmptyMapValue(c.Network.Options, "flannel_cni_image", DefaultFlannelCNIImage)

	case c.Network.Plugin == CalicoNetworkPlugin:
		setDefaultIfEmptyMapValue(c.Network.Options, "calico_cni_image", DefaultCalicoCNIImage)
		setDefaultIfEmptyMapValue(c.Network.Options, "calico_node_image", DefaultCalicoNodeImage)
		setDefaultIfEmptyMapValue(c.Network.Options, "calico_controllers_image", DefaultCalicoControllersImage)

	case c.Network.Plugin == CanalNetworkPlugin:
		setDefaultIfEmptyMapValue(c.Network.Options, "canal_cni_image", DefaultCanalCNIImage)
		setDefaultIfEmptyMapValue(c.Network.Options, "canal_node_image", DefaultCanalNodeImage)
		setDefaultIfEmptyMapValue(c.Network.Options, "canal_flannel_image", DefaultCanalFlannelImage)
	}
}

package cluster

import (
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
)

func (c *Cluster) ClusterRemove() error {
	// Remove Worker Plane
	if err := services.RemoveWorkerPlane(c.WorkerHosts, true); err != nil {
		return err
	}

	// Remove Contol Plane
	if err := services.RemoveControlPlane(c.ControlPlaneHosts, true); err != nil {
		return err
	}

	// Remove Etcd Plane
	if err := services.RemoveEtcdPlane(c.EtcdHosts); err != nil {
		return err
	}

	// Clean up all hosts
	if err := cleanUpHosts(c.ControlPlaneHosts, c.WorkerHosts, c.EtcdHosts, c.RKEImages["alpine"]); err != nil {
		return err
	}

	return pki.RemoveAdminConfig(c.LocalKubeConfigPath)
}

<<<<<<< HEAD
func cleanUpHosts(cpHosts, workerHosts, etcdHosts []*hosts.Host) error {
	allHosts := []*hosts.Host{}
=======
func cleanUpHosts(cpHosts, workerHosts, etcdHosts []hosts.Host, cleanerImage string) error {
	allHosts := []hosts.Host{}
>>>>>>> configurable_images_wip
	allHosts = append(allHosts, cpHosts...)
	allHosts = append(allHosts, workerHosts...)
	allHosts = append(allHosts, etcdHosts...)

	for _, host := range allHosts {
<<<<<<< HEAD
		if err := host.CleanUpAll(); err != nil {
=======
		if err := host.CleanUp(cleanerImage); err != nil {
>>>>>>> configurable_images_wip
			return err
		}
	}
	return nil
}

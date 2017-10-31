package services

import (
	"github.com/Sirupsen/logrus"
	"github.com/rancher/rke/hosts"
)

func RunControlPlane(masterHosts []hosts.Host, etcdHosts []hosts.Host, masterServices Services) error {
	logrus.Infof("[%s] Building up Controller Plane..", ControlRole)
	for _, host := range masterHosts {
		// run kubeapi
		err := runKubeAPI(host, etcdHosts, masterServices.KubeAPI)
		if err != nil {
			return err
		}
		// run kubecontroller
		err = runKubeController(host, masterServices.KubeController)
		if err != nil {
			return err
		}
		// run scheduler
		err = runScheduler(host, masterServices.Scheduler)
		if err != nil {
			return err
		}
	}
	logrus.Infof("[%s] Successfully started Controller Plane..", ControlRole)
	return nil
}

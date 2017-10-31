package services

import (
	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
)

type Scheduler struct {
	Version string `yaml:"version"`
	Image   string `yaml:"image"`
}

func runScheduler(host hosts.Host, schedulerService Scheduler) error {
	imageCfg, hostCfg := buildSchedulerConfig(host, schedulerService)
	err := docker.DoRunContainer(imageCfg, hostCfg, SchedulerContainerName, &host, ControlRole)
	if err != nil {
		return err
	}
	return nil
}

func buildSchedulerConfig(host hosts.Host, schedulerService Scheduler) (*container.Config, *container.HostConfig) {
	imageCfg := &container.Config{
		Image: schedulerService.Image + ":" + schedulerService.Version,
		Cmd: []string{"/hyperkube",
			"scheduler",
			"--v=2",
			"--address=0.0.0.0",
			"--master=http://" + host.IP + ":8080/"},
	}
	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "always"},
	}
	return imageCfg, hostCfg
}

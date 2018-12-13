package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/urfave/cli"
)

var commonFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "ssh-agent-auth",
		Usage: "Use SSH Agent Auth defined by SSH_AUTH_SOCK",
	},
	cli.BoolFlag{
		Name:  "ignore-docker-version",
		Usage: "Disable Docker version check",
	},
}

func resolveClusterFile(ctx *cli.Context) (string, string, error) {
	clusterFile := ctx.String("config")
	fp, err := filepath.Abs(clusterFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to lookup current directory name: %v", err)
	}
	file, err := os.Open(fp)
	if err != nil {
		return "", "", fmt.Errorf("can not find cluster configuration file: %v", err)
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %v", err)
	}
	clusterFileBuff := string(buf)
	return clusterFileBuff, clusterFile, nil
}

func setOptionsFromCLI(c *cli.Context, rkeConfig *v3.RancherKubernetesEngineConfig) (*v3.RancherKubernetesEngineConfig, error) {
	// If true... override the file.. else let file value go through
	if c.Bool("ssh-agent-auth") {
		rkeConfig.SSHAgentAuth = c.Bool("ssh-agent-auth")
	}

	if c.Bool("ignore-docker-version") {
		rkeConfig.IgnoreDockerVersion = c.Bool("ignore-docker-version")
	}

	if c.Bool("s3") {
		if rkeConfig.Services.Etcd.BackupTarget == nil {
			rkeConfig.Services.Etcd.BackupTarget = &v3.BackupTarget{}
		}
		rkeConfig.Services.Etcd.BackupTarget.S3BackupTarget = setS3OptionsFromCLI(c)
	}

	return rkeConfig, nil
}

func setS3OptionsFromCLI(c *cli.Context) *v3.S3BackupTarget {
	endpoint := c.String("s3-endpoint")
	accessKey := c.String("access-key")
	secretKey := c.String("secret-key")
	bucketName := c.String("bucket-name")
	region := c.String("region")
	var s3BackupTarget = &v3.S3BackupTarget{}
	if len(endpoint) != 0 {
		s3BackupTarget.Endpoint = endpoint
	}
	if len(accessKey) != 0 {
		s3BackupTarget.AccessKey = accessKey
	}
	if len(secretKey) != 0 {
		s3BackupTarget.SecretKey = secretKey
	}
	if len(bucketName) != 0 {
		s3BackupTarget.BucketName = bucketName
	}
	if len(region) != 0 {
		s3BackupTarget.Region = region
	}
	return s3BackupTarget
}

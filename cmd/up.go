package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/dind"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/urfave/cli"
)

var clusterFilePath string

const DINDWaitTime = 3

func UpCommand() cli.Command {
	upFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Specify an alternate cluster YAML file",
			Value:  pki.ClusterConfig,
			EnvVar: "RKE_CONFIG",
		},
		cli.BoolFlag{
			Name:  "local",
			Usage: "Deploy Kubernetes cluster locally",
		},
		cli.BoolFlag{
			Name:  "dind",
			Usage: "Deploy Kubernetes cluster in docker containers (experimental)",
		},
		cli.StringFlag{
			Name:  "dind-storage-driver",
			Usage: "Storage driver for the docker in docker containers (experimental)",
		},
		cli.BoolFlag{
			Name:  "update-only",
			Usage: "Skip idempotent deployment of control and etcd plane",
		},
		cli.BoolFlag{
			Name:  "disable-port-check",
			Usage: "Disable port check validation between nodes",
		},
		cli.BoolFlag{
			Name:  "init",
			Usage: "test init",
		},
	}

	upFlags = append(upFlags, commonFlags...)

	return cli.Command{
		Name:   "up",
		Usage:  "Bring the cluster up",
		Action: clusterUpFromCli,
		Flags:  upFlags,
	}
}

func ClusterInit(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, configDir string) error {
	log.Infof(ctx, "Initiating Kubernetes cluster")
	stateFilePath := cluster.GetStateFilePath(clusterFilePath, configDir)
	rkeFullState, _ := cluster.ReadStateFile(ctx, stateFilePath)

	kubeCluster, err := cluster.ParseCluster(ctx, rkeConfig, clusterFilePath, configDir, nil, nil, nil)
	if err != nil {
		return err
	}

	desiredState, err := cluster.RebuildState(ctx, &kubeCluster.RancherKubernetesEngineConfig, rkeFullState.DesiredState)
	if err != nil {
		return err
	}
	rkeState := cluster.RKEFullState{
		DesiredState: desiredState,
		CurrentState: rkeFullState.CurrentState,
	}
	return rkeState.WriteStateFile(ctx, stateFilePath)
}

func ClusterUp(
	ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dockerDialerFactory, localConnDialerFactory hosts.DialerFactory,
	k8sWrapTransport k8s.WrapTransport,
	local bool, configDir string, updateOnly, disablePortCheck bool) (string, string, string, string, map[string]pki.CertificatePKI, error) {

	log.Infof(ctx, "Building Kubernetes cluster")
	var APIURL, caCrt, clientCert, clientKey string

	// is tehre any chance we can store the cluster object here instead of the rke config ?
	// I can change the function signiture, should be simpler
	// No, I would stil have to parse the cluster
	clusterState, err := cluster.ReadStateFile(ctx, cluster.GetStateFilePath(clusterFilePath, configDir))
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	kubeCluster, err := cluster.InitClusterObject(ctx, clusterState.DesiredState.RancherKubernetesEngineConfig, clusterFilePath, configDir)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	err = kubeCluster.SetupDialers(ctx, dockerDialerFactory, localConnDialerFactory, k8sWrapTransport)
	// kubeCluster, err := cluster.ParseCluster(ctx, clusterState.DesiredState.RancherKubernetesEngineConfig, clusterFilePath, configDir, dockerDialerFactory, localConnDialerFactory, k8sWrapTransport)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.TunnelHosts(ctx, local)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	// 1. fix the kube config if it's broken
	// 2. connect to k8s
	// 3. get the state from k8s
	// 4. if not on k8s we get it from the nodes.
	// 5. get cluster certificates
	// 6. update etcd hosts certs
	// 7. set cluster defaults
	// 8. regenerate api certificates
	currentCluster, err := kubeCluster.NewGetClusterState(ctx, clusterState, configDir)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	if !disablePortCheck {
		if err = kubeCluster.CheckClusterPorts(ctx, currentCluster); err != nil {
			return APIURL, caCrt, clientCert, clientKey, nil, err
		}
	}

	// 0. check on the auth strategy
	// 1. if current cluster != nil copy over certs to kubeCluster
	// 1.1. if there is no pki.RequestHeaderCACertName, generate it
	// 2. fi there is no current_cluster try to fetch backup
	// 2.1 if you found backup, handle weird fucking cases
	// 3. if you don't find backup, generate new certs!
	// 4. deploy backups
	// This looks very weird now..
	err = cluster.NewSetUpAuthentication(ctx, kubeCluster, currentCluster, clusterState)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	// if len(kubeCluster.ControlPlaneHosts) > 0 {
	// 	APIURL = fmt.Sprintf("https://" + kubeCluster.ControlPlaneHosts[0].Address + ":6443")
	// }
	// clientCert = string(cert.EncodeCertPEM(kubeCluster.Certificates[pki.KubeAdminCertName].Certificate))
	// clientKey = string(cert.EncodePrivateKeyPEM(kubeCluster.Certificates[pki.KubeAdminCertName].Key))
	// caCrt = string(cert.EncodeCertPEM(kubeCluster.Certificates[pki.CACertName].Certificate))

	err = cluster.ReconcileCluster(ctx, kubeCluster, currentCluster, updateOnly)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	// update APIURL after reconcile
	if len(kubeCluster.ControlPlaneHosts) > 0 {
		APIURL = fmt.Sprintf("https://" + kubeCluster.ControlPlaneHosts[0].Address + ":6443")
	}

	err = kubeCluster.SetUpHosts(ctx, false)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	if err := kubeCluster.PrePullK8sImages(ctx); err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.DeployControlPlane(ctx)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	// Apply Authz configuration after deploying controlplane
	err = cluster.ApplyAuthzResources(ctx, kubeCluster.RancherKubernetesEngineConfig, clusterFilePath, configDir, k8sWrapTransport)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	// 1. save cluster certificates
	// 2. save cluster state
	//err = kubeCluster.SaveClusterState(ctx, &kubeCluster.RancherKubernetesEngineConfig)
	err = kubeCluster.UpdateClusterSate(ctx, clusterState)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.DeployWorkerPlane(ctx)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	if err = kubeCluster.CleanDeadLogs(ctx); err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.SyncLabelsAndTaints(ctx, currentCluster)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = cluster.ConfigureCluster(ctx, kubeCluster.RancherKubernetesEngineConfig, kubeCluster.Certificates, clusterFilePath, configDir, k8sWrapTransport, false)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	if err := checkAllIncluded(kubeCluster); err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	log.Infof(ctx, "Finished building Kubernetes cluster successfully")
	return APIURL, caCrt, clientCert, clientKey, kubeCluster.Certificates, nil
}

func checkAllIncluded(cluster *cluster.Cluster) error {
	if len(cluster.InactiveHosts) == 0 {
		return nil
	}

	var names []string
	for _, host := range cluster.InactiveHosts {
		names = append(names, host.Address)
	}

	return fmt.Errorf("Provisioning incomplete, host(s) [%s] skipped because they could not be contacted", strings.Join(names, ","))
}

func clusterUpFromCli(ctx *cli.Context) error {
	if ctx.Bool("local") {
		return clusterUpLocal(ctx)
	}
	if ctx.Bool("dind") {
		return clusterUpDind(ctx)
	}
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		return fmt.Errorf("Failed to resolve cluster file: %v", err)
	}
	clusterFilePath = filePath

	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return fmt.Errorf("Failed to parse cluster file: %v", err)
	}

	rkeConfig, err = setOptionsFromCLI(ctx, rkeConfig)
	if err != nil {
		return err
	}
	updateOnly := ctx.Bool("update-only")
	disablePortCheck := ctx.Bool("disable-port-check")
	if ctx.Bool("init") {
		return ClusterInit(context.Background(), rkeConfig, "")
	}
	if err := ClusterInit(context.Background(), rkeConfig, ""); err != nil {
		return err
	}
	_, _, _, _, _, err = ClusterUp(context.Background(), rkeConfig, nil, nil, nil, false, "", updateOnly, disablePortCheck)
	return err
}

func clusterUpLocal(ctx *cli.Context) error {
	var rkeConfig *v3.RancherKubernetesEngineConfig
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		log.Infof(context.Background(), "Failed to resolve cluster file, using default cluster instead")
		rkeConfig = cluster.GetLocalRKEConfig()
	} else {
		clusterFilePath = filePath
		rkeConfig, err = cluster.ParseConfig(clusterFile)
		if err != nil {
			return fmt.Errorf("Failed to parse cluster file: %v", err)
		}
		rkeConfig.Nodes = []v3.RKEConfigNode{*cluster.GetLocalRKENodeConfig()}
	}

	rkeConfig.IgnoreDockerVersion = ctx.Bool("ignore-docker-version")

	_, _, _, _, _, err = ClusterUp(context.Background(), rkeConfig, nil, hosts.LocalHealthcheckFactory, nil, true, "", false, false)
	return err
}

func clusterUpDind(ctx *cli.Context) error {
	// get dind config
	rkeConfig, disablePortCheck, dindStorageDriver, err := getDindConfig(ctx)
	if err != nil {
		return err
	}
	// setup dind environment
	if err = createDINDEnv(context.Background(), rkeConfig, dindStorageDriver); err != nil {
		return err
	}
	// start cluster
	_, _, _, _, _, err = ClusterUp(context.Background(), rkeConfig, hosts.DindConnFactory, hosts.DindHealthcheckConnFactory, nil, false, "", false, disablePortCheck)
	return err
}

func getDindConfig(ctx *cli.Context) (*v3.RancherKubernetesEngineConfig, bool, string, error) {
	disablePortCheck := ctx.Bool("disable-port-check")
	dindStorageDriver := ctx.String("dind-storage-driver")

	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		return nil, disablePortCheck, "", fmt.Errorf("Failed to resolve cluster file: %v", err)
	}
	clusterFilePath = filePath

	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return nil, disablePortCheck, "", fmt.Errorf("Failed to parse cluster file: %v", err)
	}

	rkeConfig, err = setOptionsFromCLI(ctx, rkeConfig)
	if err != nil {
		return nil, disablePortCheck, "", err
	}
	// Setting conntrack max for kubeproxy to 0
	if rkeConfig.Services.Kubeproxy.ExtraArgs == nil {
		rkeConfig.Services.Kubeproxy.ExtraArgs = make(map[string]string)
	}
	rkeConfig.Services.Kubeproxy.ExtraArgs["conntrack-max-per-core"] = "0"

	return rkeConfig, disablePortCheck, dindStorageDriver, nil
}

func createDINDEnv(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dindStorageDriver string) error {
	for i := range rkeConfig.Nodes {
		address, err := dind.StartUpDindContainer(ctx, rkeConfig.Nodes[i].Address, dind.DINDNetwork, dindStorageDriver)
		if err != nil {
			return err
		}
		if rkeConfig.Nodes[i].HostnameOverride == "" {
			rkeConfig.Nodes[i].HostnameOverride = rkeConfig.Nodes[i].Address
		}
		rkeConfig.Nodes[i].Address = address
	}
	time.Sleep(DINDWaitTime * time.Second)
	return nil
}

package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
)

type KubernetesCollectorConfig struct {
	EnableClusterAndNodePoolInfoMetric bool `json:"enableClusterAndNodePoolInfoMetric"`
}

type KubernetesCollector struct {
	account *gcp.Account
	config KubernetesCollectorConfig

	Info          *prometheus.Desc
	NodePoolsInfo *prometheus.Desc
	Nodes         *prometheus.Desc
	Up            *prometheus.Desc
}

func NewKubernetesCollector(account *gcp.Account, config string) *KubernetesCollector {
	var collectorConfig = KubernetesCollectorConfig{EnableClusterAndNodePoolInfoMetric: false}

	if err := json.Unmarshal([]byte(config), &collectorConfig); err != nil {
		log.Println("Unable to decode JSON:", err)
	} else {
		log.Printf("[KubernetesCollector] Config: %+v\n", collectorConfig)
	}

	fqName := name("kubernetes_engine")
	labelKeys := []string{"project", "name", "location", "version"}

	return &KubernetesCollector{
		account: account,
		config:  collectorConfig,
		Up: prometheus.NewDesc(
			fqName("cluster_up"),
			"1 if the cluster is running, 0 otherwise",
			labelKeys, nil,
		),
		Info: prometheus.NewDesc(
			fqName("cluster_info"),
			"Cluster control plane information. 1 if the cluster is running, 0 otherwise",
			append(labelKeys, "id", "mode", "endpoint", "network", "subnetwork",
				"initial_cluster_version", "node_pools_count"),
			nil,
		),
		Nodes: prometheus.NewDesc(
			fqName("cluster_nodes"),
			"Number of nodes currently in the cluster",
			labelKeys, nil,
		),
		NodePoolsInfo: prometheus.NewDesc(
			fqName("cluster_node_pools_info"),
			"Cluster Node Pools Information. 1 if the Node Pool is running, 0 otherwise",
			append(labelKeys, "etag", "cluster_id", "autoscaling", "disk_size_gb",
				"disk_type", "image_type", "machine_type", "locations", "spot", "preemptible"),
			nil,
		),
	}
}

func (c *KubernetesCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	containerService, err := container.NewService(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		wg.Add(1)
		go func(p *cloudresourcemanager.Project) {
			defer wg.Done()
			c.collectProjectMetrics(ctx, containerService, p, ch)
		}(p)
	}
	wg.Wait()
}

func (c *KubernetesCollector) collectProjectMetrics(ctx context.Context, containerService *container.Service,
	p *cloudresourcemanager.Project, ch chan<- prometheus.Metric) {

	log.Printf("[KubernetesCollector:go] Project: %s", p.ProjectId)
	parent := fmt.Sprintf("projects/%s/locations/-", p.ProjectId)
	resp, err := containerService.Projects.Locations.Clusters.List(parent).Context(ctx).Do()

	if err != nil {
		if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusForbidden {
			log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
			return
		}
		log.Println("Google API Error:", err)
		return
	}

	for _, cluster := range resp.Clusters {
		c.collectClusterMetrics(p, cluster, ch)
	}
}

func (c *KubernetesCollector) collectClusterMetrics(p *cloudresourcemanager.Project, cluster *container.Cluster,
	ch chan<- prometheus.Metric) {

	log.Printf("[KubernetesCollector] cluster: %s", cluster.Name)

	clusterStatus := 0.0
	if cluster.Status == "RUNNING" {
		clusterStatus = 1.0
	}

	ch <- prometheus.MustNewConstMetric(c.Up, prometheus.GaugeValue, clusterStatus,
		p.ProjectId, cluster.Name, cluster.Location, cluster.CurrentMasterVersion)

	ch <- prometheus.MustNewConstMetric(c.Nodes, prometheus.GaugeValue, float64(cluster.CurrentNodeCount),
		p.ProjectId, cluster.Name, cluster.Location, cluster.CurrentNodeVersion)

	if c.config.EnableClusterAndNodePoolInfoMetric {
		c.collectNodePoolMetrics(p, cluster, ch, clusterStatus)
	}
}

func (c *KubernetesCollector) collectNodePoolMetrics(p *cloudresourcemanager.Project, cluster *container.Cluster,
	ch chan<- prometheus.Metric, clusterStatus float64) {

	if cluster.NodePools == nil || len(cluster.NodePools) == 0 {
		return
	}

	nodePoolsSize := strconv.Itoa(len(cluster.NodePools))
	clusterMode := "Standard"

	if cluster.Autopilot != nil && cluster.Autopilot.Enabled {
		clusterMode = "Autopilot"
	}

	ch <- prometheus.MustNewConstMetric(c.Info, prometheus.GaugeValue, clusterStatus,
		p.ProjectId, cluster.Name, cluster.Location, cluster.CurrentMasterVersion,
		cluster.Id, clusterMode, cluster.Endpoint, cluster.Network, cluster.Subnetwork,
		cluster.InitialClusterVersion, nodePoolsSize)

	for _, nodePool := range cluster.NodePools {
		nodePoolStatus := 0.0
		if nodePool.Status == "RUNNING" {
			nodePoolStatus = 1.0
		}

		boolToString := func(b bool) string { return strconv.FormatBool(b) }

		ch <- prometheus.MustNewConstMetric(c.NodePoolsInfo, prometheus.GaugeValue, nodePoolStatus,
			p.ProjectId, nodePool.Name, cluster.Location, nodePool.Version, nodePool.Etag, cluster.Id,
			boolToString(nodePool.Autoscaling.Enabled),
			strconv.FormatInt(nodePool.Config.DiskSizeGb, 10), nodePool.Config.DiskType,
			nodePool.Config.ImageType, nodePool.Config.MachineType,
			strings.Join(nodePool.Locations, ","),
			boolToString(nodePool.Config.Spot),
			boolToString(nodePool.Config.Preemptible))
	}
}

func (c *KubernetesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Info
	ch <- c.NodePoolsInfo
	ch <- c.Nodes
	ch <- c.Up
}

package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/DazWilkin/gcp-exporter/internal"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
)

type GKECollectorConfig struct {
	EnableExtendedMetrics    bool
	ExtraLabelsClusterInfo   string
	ExtraLabelsNodePoolsInfo string
}

// GKECollector collects GKE cluster metrics
type GKECollector struct {
	account                          *gcp.Account
	enableExtendedMetrics            bool
	extraLabelsClusterInfoExtended   internal.ExtraLabel
	extraLabelsNodePoolsInfoExtended internal.ExtraLabel

	Info          *prometheus.Desc
	NodePoolsInfo *prometheus.Desc
	Nodes         *prometheus.Desc
	Up            *prometheus.Desc
}

// NewGKECollector initializes the GKE collector
func NewGKECollector(account *gcp.Account, config GKECollectorConfig) *GKECollector {
	fqName := name("gke")
	labelKeys := []string{"project", "name", "location", "version"}

	// Initialize extra labels with order preservation
	var extraLabelsClusterInfo internal.ExtraLabel
	var extraLabelsNodePoolsInfo internal.ExtraLabel

	infoLabelKeys := append(labelKeys, []string{"id", "mode", "endpoint", "network", "subnetwork",
		"initial_cluster_version", "node_pools_count"}...)
	nodePoolsInfoLabelKeys := append(labelKeys, []string{"etag", "cluster_id", "autoscaling", "disk_size_gb",
		"disk_type", "image_type", "machine_type", "locations", "spot", "preemptible"}...)

	if len(config.ExtraLabelsClusterInfo) > 0 {
		extraLabelsClusterInfo = internal.ProcessExtraLabels(config.ExtraLabelsClusterInfo)
		infoLabelKeys = append(infoLabelKeys, internal.GetLabelNamesFromExtraLabels(extraLabelsClusterInfo)...)
	}

	if len(config.ExtraLabelsNodePoolsInfo) > 0 {
		extraLabelsNodePoolsInfo = internal.ProcessExtraLabels(config.ExtraLabelsNodePoolsInfo)
		nodePoolsInfoLabelKeys = append(nodePoolsInfoLabelKeys, internal.GetLabelNamesFromExtraLabels(extraLabelsNodePoolsInfo)...)
	}

	return &GKECollector{
		account:                          account,
		enableExtendedMetrics:            config.EnableExtendedMetrics,
		extraLabelsClusterInfoExtended:   extraLabelsClusterInfo,
		extraLabelsNodePoolsInfoExtended: extraLabelsNodePoolsInfo,

		Up: prometheus.NewDesc(
			fqName("up"),
			"1 if the cluster is running, 0 otherwise",
			labelKeys, nil,
		),
		Info: prometheus.NewDesc(
			fqName("info"),
			"Cluster control plane information. 1 if the cluster is running, 0 otherwise",
			infoLabelKeys,
			nil,
		),
		Nodes: prometheus.NewDesc(
			fqName("nodes"),
			"Number of nodes currently in the cluster",
			labelKeys, nil,
		),
		NodePoolsInfo: prometheus.NewDesc(
			fqName("node_pools_info"),
			"Cluster Node Pools Information. 1 if the Node Pool is running, 0 otherwise",
			nodePoolsInfoLabelKeys,
			nil,
		),
	}
}

// Collect fetches metrics from GKE clusters and pushes them to the Prometheus channel
func (c *GKECollector) Collect(ch chan<- prometheus.Metric) {
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

// collectProjectMetrics collects metrics for each project
func (c *GKECollector) collectProjectMetrics(ctx context.Context, containerService *container.Service,
	p *cloudresourcemanager.Project, ch chan<- prometheus.Metric) {

	log.Printf("[GKECollector:go] Project: %s", p.ProjectId)
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

// collectClusterMetrics collects metrics for each cluster
func (c *GKECollector) collectClusterMetrics(p *cloudresourcemanager.Project, cluster *container.Cluster,
	ch chan<- prometheus.Metric) {

	log.Printf("[GKECollector] cluster: %s", cluster.Name)

	clusterStatus := 0.0
	if cluster.Status == "RUNNING" {
		clusterStatus = 1.0
	}

	// Collect the base metrics
	ch <- prometheus.MustNewConstMetric(c.Up, prometheus.GaugeValue, clusterStatus,
		p.ProjectId, cluster.Name, cluster.Location, cluster.CurrentMasterVersion)

	ch <- prometheus.MustNewConstMetric(c.Nodes, prometheus.GaugeValue, float64(cluster.CurrentNodeCount),
		p.ProjectId, cluster.Name, cluster.Location, cluster.CurrentNodeVersion)

	// Collect extended metrics if enabled
	if c.enableExtendedMetrics {
		c.collectExtendedMetrics(p, cluster, ch, clusterStatus)
	}
}

// collectExtendedMetrics collects the extended metrics for each cluster, including extra labels
func (c *GKECollector) collectExtendedMetrics(p *cloudresourcemanager.Project, cluster *container.Cluster,
	ch chan<- prometheus.Metric, clusterStatus float64) {

	if len(cluster.NodePools) == 0 {
		return
	}

	nodePoolsSize := strconv.Itoa(len(cluster.NodePools))
	clusterMode := "Standard"

	if cluster.Autopilot != nil && cluster.Autopilot.Enabled {
		clusterMode = "Autopilot"
	}

	labelValuesClusterInfo := []string{
		p.ProjectId, cluster.Name, cluster.Location, cluster.CurrentMasterVersion,
		cluster.Id, clusterMode, cluster.Endpoint, cluster.Network, cluster.Subnetwork,
		cluster.InitialClusterVersion, nodePoolsSize,
	}

	// Add the extra labels to cluster info
	labelValuesClusterInfo = append(labelValuesClusterInfo, internal.GetExtraLabelsValues(cluster.ResourceLabels, c.extraLabelsClusterInfoExtended)...)

	// Collect the extended metrics for the cluster
	ch <- prometheus.MustNewConstMetric(c.Info, prometheus.GaugeValue, clusterStatus, labelValuesClusterInfo...)

	for _, nodePool := range cluster.NodePools {
		nodePoolStatus := 0.0
		if nodePool.Status == "RUNNING" {
			nodePoolStatus = 1.0
		}

		nodePoolConfigSpec := nodePool.Config
		boolToString := func(b bool) string { return strconv.FormatBool(b) }

		labelValuesNodePoolInfo := []string{
			p.ProjectId, nodePool.Name, cluster.Location, nodePool.Version, nodePool.Etag, cluster.Id,
			boolToString(nodePool.Autoscaling.Enabled),
			strconv.FormatInt(nodePoolConfigSpec.DiskSizeGb, 10), nodePoolConfigSpec.DiskType,
			nodePoolConfigSpec.ImageType, nodePoolConfigSpec.MachineType,
			strings.Join(nodePool.Locations, ","),
			boolToString(nodePoolConfigSpec.Spot),
			boolToString(nodePoolConfigSpec.Preemptible),
		}
		// Add the extra labels to node pool info
		labelValuesNodePoolInfo = append(labelValuesNodePoolInfo, internal.GetExtraLabelsValues(nodePoolConfigSpec.ResourceLabels, c.extraLabelsNodePoolsInfoExtended)...)

		// Collect the extended metrics for the node pool
		ch <- prometheus.MustNewConstMetric(c.NodePoolsInfo, prometheus.GaugeValue, nodePoolStatus, labelValuesNodePoolInfo...)
	}
}

// Describe registers the metrics descriptions
func (c *GKECollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Info
	ch <- c.NodePoolsInfo
	ch <- c.Nodes
	ch <- c.Up
}

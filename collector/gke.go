package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
)

type GKECollector struct {
	account          *gcp.Account
	containerService *container.Service

	enableExtendedMetrics bool

	EndOfStandardSupportTimestamp         *prometheus.Desc
	Info                                  *prometheus.Desc
	NodePoolEndOfStandardSupportTimestamp *prometheus.Desc
	NodePoolInfo                         *prometheus.Desc
	Nodes                                 *prometheus.Desc
	Up                                    *prometheus.Desc
}

func NewGKECollector(account *gcp.Account, enableExtendedMetrics bool) (*GKECollector, error) {
	subsystem := "gke"
	labelKeys := []string{"project", "name", "location", "version"}

	ctx := context.Background()
	containerService, err := container.NewService(ctx)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &GKECollector{
		account:          account,
		containerService: containerService,

		enableExtendedMetrics: enableExtendedMetrics,

		Up: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "up"),
			"1 if the cluster is running, 0 otherwise",
			labelKeys, nil,
		),
		Info: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "info"),
			"Cluster control plane information. 1 if the cluster is running, 0 otherwise",
			append(labelKeys, "id", "mode", "endpoint", "network", "subnetwork",
				"initial_cluster_version", "node_pools_count"),
			nil,
		),
		EndOfStandardSupportTimestamp: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "endof_standard_support_timestamp"),
			"Cluster control plane version standard support End of Life timestamp",
			[]string{"cluster_id"},
			nil,
		),
		Nodes: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "nodes"),
			"Number of nodes currently in the cluster",
			labelKeys, nil,
		),
		NodePoolInfo: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "node_pool_info"),
			"Cluster Node Pools Information. 1 if the Node Pool is running, 0 otherwise",
			append(labelKeys, "etag", "cluster_id", "autoscaling", "disk_size_gb",
				"disk_type", "image_type", "machine_type", "locations", "spot", "preemptible"),
			nil,
		),
		NodePoolEndOfStandardSupportTimestamp: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "node_pool_endof_standard_support_timestamp"),
			"Cluster Node Pools version standard support End of Life timestamp",
			[]string{"etag", "cluster_id"},
			nil,
		),
	}, nil
}

func (c *GKECollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		wg.Add(1)
		go func(p *cloudresourcemanager.Project) {
			defer wg.Done()
			c.collectProjectMetrics(ctx, c.containerService, p, ch)
		}(p)
	}
	wg.Wait()
}

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
		c.collectClusterMetrics(ctx, p, cluster, ch, containerService)
	}
}

func (c *GKECollector) collectClusterMetrics(ctx context.Context, p *cloudresourcemanager.Project, cluster *container.Cluster,
	ch chan<- prometheus.Metric, containerService *container.Service) {

	log.Printf("[GKECollector] cluster: %s", cluster.Name)

	clusterStatus := 0.0
	if cluster.Status == "RUNNING" {
		clusterStatus = 1.0
	}

	ch <- prometheus.MustNewConstMetric(c.Up, prometheus.GaugeValue, clusterStatus,
		p.ProjectId, cluster.Name, cluster.Location, cluster.CurrentMasterVersion)

	ch <- prometheus.MustNewConstMetric(c.Nodes, prometheus.GaugeValue, float64(cluster.CurrentNodeCount),
		p.ProjectId, cluster.Name, cluster.Location, cluster.CurrentNodeVersion)

	if c.enableExtendedMetrics {
		c.collectClusterExtendedMetrics(ctx, p, cluster, ch, containerService)
		c.collectNodePoolExtendedMetrics(ctx, p, cluster, ch, containerService, clusterStatus)
	}
}

func (c *GKECollector) collectClusterExtendedMetrics(ctx context.Context, p *cloudresourcemanager.Project,
	cluster *container.Cluster, ch chan<- prometheus.Metric, containerService *container.Service) {

	parent := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", p.ProjectId, cluster.Location, cluster.Name)
	resp, err := containerService.Projects.Locations.Clusters.FetchClusterUpgradeInfo(parent).Context(ctx).Do()

	if err != nil {
		if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusForbidden {
			log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
			continue
		}
		log.Println("Google API Error:", err)
		continue
	}

	ch <- prometheus.MustNewConstMetric(c.EndOfStandardSupportTimestamp, prometheus.GaugeValue,
		float64(DateToUnix(resp.EndOfStandardSupportTimestamp)), cluster.Id)
}

func (c *GKECollector) collectNodePoolExtendedMetrics(ctx context.Context, p *cloudresourcemanager.Project,
	cluster *container.Cluster, ch chan<- prometheus.Metric, containerService *container.Service, clusterStatus float64) {

	if len(cluster.NodePools) == 0 {
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
		nodePollAutoScaling := false

		if nodePool.Status == "RUNNING" {
			nodePoolStatus = 1.0
		}

		if nodePool.Autoscaling != nil && nodePool.Autoscaling.Enabled {
			nodePollAutoScaling = nodePool.Autoscaling.Enabled
		}

		boolToString := func(b bool) string { return strconv.FormatBool(b) }

		ch <- prometheus.MustNewConstMetric(c.NodePoolInfo, prometheus.GaugeValue, nodePoolStatus,
			p.ProjectId, nodePool.Name, cluster.Location, nodePool.Version, nodePool.Etag, cluster.Id,
			boolToString(nodePollAutoScaling),
			strconv.FormatInt(nodePool.Config.DiskSizeGb, 10), nodePool.Config.DiskType,
			nodePool.Config.ImageType, nodePool.Config.MachineType,
			strings.Join(nodePool.Locations, ","),
			boolToString(nodePool.Config.Spot),
			boolToString(nodePool.Config.Preemptible))

		parent := fmt.Sprintf("projects/%s/locations/%s/clusters/%s/nodePools/%s", p.ProjectId,
			cluster.Location, cluster.Name, nodePool.Name)
		resp, err := containerService.Projects.Locations.Clusters.NodePools.FetchNodePoolUpgradeInfo(parent).Context(ctx).Do()

		if err != nil {
			if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusForbidden {
				log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
				continue
			}
			log.Println("Google API Error:", err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(c.NodePoolEndOfStandardSupportTimestamp, prometheus.GaugeValue,
			float64(DateToUnix(resp.EndOfStandardSupportTimestamp)), nodePool.Etag, cluster.Id)
	}
}

func (c *GKECollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.EndOfStandardSupportTimestamp
	ch <- c.Info
	ch <- c.NodePoolEndOfStandardSupportTimestamp
	ch <- c.NodePoolInfo
	ch <- c.Nodes
	ch <- c.Up
}

func DateToUnix(dateStr string) int64 {
	layout := "2006-01-02"
	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return time.Now().Unix()
	}
	return t.Unix()
}

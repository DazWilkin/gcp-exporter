package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
)

// KubernetesCollector represents Kubernetes Engine
type KubernetesCollector struct {
	account *gcp.Account

	Up    *prometheus.Desc
	Nodes *prometheus.Desc
}

// NewKubernetesCollector creates a new KubernetesCollector
func NewKubernetesCollector(account *gcp.Account) *KubernetesCollector {
	fqName := name("kubernetes_engine")
	labelKeys := []string{
		"name",
		"location",
		"version",
	}
	return &KubernetesCollector{
		account: account,

		Up: prometheus.NewDesc(
			fqName("cluster_up"),
			"1 if the cluster is running, 0 otherwise",
			labelKeys,
			nil,
		),
		Nodes: prometheus.NewDesc(
			fqName("cluster_nodes"),
			"Number of nodes currently in the cluster",
			labelKeys,
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *KubernetesCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	containerService, err := container.NewService(ctx)
	if err != nil {
		log.Println(err)
	}

	// Enumerate all of the projects
	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		wg.Add(1)
		go func(p *cloudresourcemanager.Project) {
			defer wg.Done()
			log.Printf("[KubernetesCollector:go] Project: %s", p.ProjectId)
			parent := fmt.Sprintf("projects/%s/locations/-", p.ProjectId)
			resp, err := containerService.Projects.Locations.Clusters.List(parent).Context(ctx).Do()
			if err != nil {
				if e, ok := err.(*googleapi.Error); ok {
					if e.Code == http.StatusForbidden {
						// Probably (!) Kubernetes Engine API has not been enabled for Project (p)
						return
					}

					log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
					return
				}

				log.Println(err)
				return
			}

			for _, cluster := range resp.Clusters {
				log.Printf("[KubernetesCollector] cluster: %s", cluster.Name)
				ch <- prometheus.MustNewConstMetric(
					c.Up,
					prometheus.CounterValue,
					func(c *container.Cluster) (result float64) {
						if c.Status == "RUNNING" {
							result = 1.0
						}
						return result
					}(cluster),
					[]string{
						cluster.Name,
						cluster.Location,
						cluster.CurrentNodeVersion,
					}...,
				)
				ch <- prometheus.MustNewConstMetric(
					c.Nodes,
					prometheus.GaugeValue,
					float64(cluster.CurrentNodeCount),
					[]string{
						cluster.Name,
						cluster.Location,
						cluster.CurrentNodeVersion,
					}...,
				)
			}
		}(p)
	}
	wg.Wait()

}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *KubernetesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Up
	ch <- c.Nodes
}

package collector

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/storage/v1"
)

// StorageCollector represents Cloud Storage
type StorageCollector struct {
	client   *http.Client
	projects []*cloudresourcemanager.Project

	Buckets *prometheus.Desc
}

// NewStorageCollector returns a StorageCollector
func NewStorageCollector(client *http.Client, projects []*cloudresourcemanager.Project) *StorageCollector {
	fqName := name("storage")
	return &StorageCollector{
		client:   client,
		projects: projects,

		Buckets: prometheus.NewDesc(
			fqName("buckets"),
			"Number of buckets",
			[]string{
				"project",
				// "region",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector inteface and is used to collect metrics
func (c *StorageCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	storageService, err := storage.New(c.client)
	if err != nil {
		log.Println(err)
		return
	}

	// Enumerate all of the projects
	var wg sync.WaitGroup
	for _, p := range c.projects {
		wg.Add(1)
		go func(p *cloudresourcemanager.Project) {
			defer wg.Done()
			log.Printf("[StorageCollector] Project: %s", p.ProjectId)
			resp, err := storageService.Buckets.List(p.ProjectId).MaxResults(500).Context(ctx).Do()
			if err != nil {
				log.Println(err)
				return
			}
			if resp.NextPageToken != "" {
				log.Println("[StorageCollector] Some buckets are being excluded from the results")
			}
			// for _, b := range resp.Items {
			// }
			ch <- prometheus.MustNewConstMetric(
				c.Buckets,
				prometheus.GaugeValue,
				float64(len(resp.Items)),
				[]string{
					p.ProjectId,
				}...,
			)
		}(p)
	}
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *StorageCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Buckets
}

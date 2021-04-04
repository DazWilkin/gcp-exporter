package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/run/v1"
)

// CloudRunCollector represents Cloud Run
type CloudRunCollector struct {
	client   *http.Client
	projects []*cloudresourcemanager.Project

	Services *prometheus.Desc
}

// NewCloudRunCollector returns a new CloudRunCollector
func NewCloudRunCollector(client *http.Client, projects []*cloudresourcemanager.Project) *CloudRunCollector {
	fqName := name("cloudrun")
	return &CloudRunCollector{
		client:   client,
		projects: projects,

		Services: prometheus.NewDesc(
			fqName("services"),
			"Number of services",
			[]string{
				"project",
				// "region",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *CloudRunCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	opts := []option.ClientOption{}
	cloudrunService, err := run.NewService(ctx, opts...)
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
			log.Printf("[CloudRunCollector] Project: %s", p.ProjectId)

			// ListServicesResponse may (!) contain Metadata
			// If Metadata is presnet, it may (!) contain Continue iff there's more data
			// https://pkg.go.dev/google.golang.org/api@v0.43.0/run/v1#ListServicesResponse
			// https://pkg.go.dev/google.golang.org/api@v0.43.0/run/v1#ListMeta

			rqst := cloudrunService.Namespaces.Services.List(Parent(p.ProjectId))

			// Do request at least once
			continue_ := ""
			count := 0
			for {
				rqst.Continue(continue_)
				resp, err := rqst.Do()
				if err != nil {
					if e, ok := err.(*googleapi.Error); ok {
						if e.Code == http.StatusForbidden {
							// Probably (!) Cloud Run Admin API has not been used in this project
							return
						}
					}
					log.Println(err)
					return
				}

				pageSize := len(resp.Items)
				count += pageSize

				if resp.Metadata != nil {
					// If there's Metadata, update continue_
					continue_ = resp.Metadata.Continue
				} else {
					// Otherwise, we're done
					break
				}
			}

			if count != 0 {
				ch <- prometheus.MustNewConstMetric(
					c.Services,
					prometheus.GaugeValue,
					float64(count),
					[]string{
						p.ProjectId,
					}...,
				)
			}
		}(p)
	}
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *CloudRunCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Services
}
func Parent(project string) string {
	return fmt.Sprintf("namespaces/%s", project)
}

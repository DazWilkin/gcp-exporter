package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/run/v1"
)

type CloudRunCollector struct {
	client   *http.Client
	projects []*cloudresourcemanager.Project

	Services *prometheus.Desc
}

func NewCloudRunCollector(client *http.Client, projects []*cloudresourcemanager.Project) *CloudRunCollector {
	fqName := name("cloud_run")
	return &CloudRunCollector{
		client:   client,
		projects: projects,

		Services: prometheus.NewDesc(
			fqName("services"),
			"Number of services",
			[]string{
				"project",
				"region",
			},
			nil,
		),
	}
}

func (c *CloudRunCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	cloudrunService, err := run.New(c.client)
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

			rqst := cloudrunService.Namespaces.Services.List(Parent(p.ProjectId))
			resp, err := rqst.Do()
			if err != nil {
				log.Println(err)
				return
			}
			count := len(resp.Items)
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
}

func (c *CloudRunCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Services
}
func Parent(project string) string {
	return fmt.Sprintf("namespaces/%s", project)
}
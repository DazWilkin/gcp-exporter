package collector

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/servicemanagement/v1"
)

var (
	_ prometheus.Collector = (*EndpointsCollector)(nil)
)

// EndpointsCollector represents Services Management services
type EndpointsCollector struct {
	account *gcp.Account

	Services *prometheus.Desc
}

// NewEndpointsCollector returns a new ServiceManagementCollector
func NewEndpointsCollector(account *gcp.Account) *EndpointsCollector {
	fqName := name("cloud_endpoints")
	return &EndpointsCollector{
		account: account,

		Services: prometheus.NewDesc(
			fqName("services"),
			"Number of Cloud Endpoints services",
			[]string{

				"project",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *EndpointsCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	servicemanagementService, err := servicemanagement.NewService(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	// Enumerate all of the projects
	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		wg.Add(1)
		go func(p *cloudresourcemanager.Project) {
			defer wg.Done()
			log.Printf("[ServiceManagementCollector] Project: %s", p.ProjectId)

			// Uses Service Management API but filters by the services
			// That have this project ID as their Producer Project ID
			// See: https://servicemanagement.googleapis.com/v1/services
			rqst := servicemanagementService.Services.List().ProducerProjectId(p.ProjectId)

			services := 0

			for {
				resp, err := rqst.Do()
				if err != nil {
					if e, ok := err.(*googleapi.Error); ok {
						if e.Code == http.StatusForbidden {
							// Probably Service Management API has not been enabled for Project (p)
							return
						}

						log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
						return
					}

					log.Println(err)
					return
				}

				services += len(resp.Services)

				// If there are no more pages, we're done
				if resp.NextPageToken == "" {
					break
				}

				// Otherwise, next page
				rqst = rqst.PageToken(resp.NextPageToken)
			}

			ch <- prometheus.MustNewConstMetric(
				c.Services,
				prometheus.GaugeValue,
				float64(services),
				[]string{
					p.ProjectId,
				}...,
			)
		}(p)
	}
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *EndpointsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Services
}

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
	"google.golang.org/api/googleapi"
	"google.golang.org/api/run/v1"
)

var (
	_ prometheus.Collector = (*CloudRunCollector)(nil)
)

// CloudRunCollector represents Cloud Run
type CloudRunCollector struct {
	account *gcp.Account

	Jobs     *prometheus.Desc
	Services *prometheus.Desc
}

// NewCloudRunCollector returns a new CloudRunCollector
func NewCloudRunCollector(account *gcp.Account) *CloudRunCollector {
	fqName := name("cloud_run")
	return &CloudRunCollector{
		account: account,

		Jobs: prometheus.NewDesc(
			fqName("jobs"),
			"Number of Jobs",
			[]string{
				"project",
				// "region",
			},
			nil,
		),
		Services: prometheus.NewDesc(
			fqName("services"),
			"Number of Services",
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
	cloudrunService, err := run.NewService(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	// Enumerate all of the projects
	// WaitGroup is used for project Services|Jobs
	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		log.Printf("[CloudRunCollector] Project: %s", p.ProjectId)

		parent := fmt.Sprintf("namespaces/%s", p.ProjectId)

		// Cloud Run services
		wg.Add(1)
		go func(p *cloudresourcemanager.Project) {
			defer wg.Done()

			// ListServicesResponse may (!) contain Metadata
			// If Metadata is presnet, it may (!) contain Continue iff there's more data
			// https://pkg.go.dev/google.golang.org/api@v0.43.0/run/v1#ListServicesResponse
			// https://pkg.go.dev/google.golang.org/api@v0.43.0/run/v1#ListMeta

			rqst := cloudrunService.Namespaces.Services.List(parent)

			// Do request at least once
			cont := ""
			count := 0
			for {
				rqst.Continue(cont)
				resp, err := rqst.Do()
				if err != nil {
					if e, ok := err.(*googleapi.Error); ok {
						if e.Code == http.StatusForbidden {
							// Probably (!) Cloud Run Admin API has not been used in this project
							return
						}

						log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
						return
					}

					log.Println(err)
					return
				}

				pageSize := len(resp.Items)
				count += pageSize

				if resp.Metadata != nil {
					// If there's Metadata, update cont
					cont = resp.Metadata.Continue
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

		// Cloud Run jobs
		wg.Add(1)
		go func(p *cloudresourcemanager.Project) {
			defer wg.Done()

			rqst := cloudrunService.Namespaces.Jobs.List(parent)

			// Do request at least once
			cont := ""
			count := 0
			for {
				rqst.Continue(cont)
				resp, err := rqst.Do()
				if err != nil {
					if e, ok := err.(*googleapi.Error); ok {
						if e.Code == http.StatusForbidden {
							// Probably (!) Cloud Run Admin API has not been used in this project
							return
						}

						log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
						return
					}

					log.Println(err)
					return
				}

				pageSize := len(resp.Items)
				count += pageSize

				if resp.Metadata != nil {
					// If there's Metadata, update cont
					cont = resp.Metadata.Continue
				} else {
					// We're done
					break
				}
			}
			if count != 0 {
				ch <- prometheus.MustNewConstMetric(
					c.Jobs,
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
	ch <- c.Jobs
}

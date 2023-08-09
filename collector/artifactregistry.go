package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"

	artifactregistry "google.golang.org/api/artifactregistry/v1beta2"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	_ prometheus.Collector = (*ArtifactRegistryCollector)(nil)
)

// ArtifactRegistryCollector represents an Artifact Registry
type ArtifactRegistryCollector struct {
	account *gcp.Account

	Registries *prometheus.Desc
	Locations  *prometheus.Desc
	Formats    *prometheus.Desc
}

// NewArtifactRegistryCollector returns a new ArtifactRegistryCollector
func NewArtifactRegistryCollector(account *gcp.Account) *ArtifactRegistryCollector {
	fqName := name("artifact_registry")
	return &ArtifactRegistryCollector{
		account: account,

		Registries: prometheus.NewDesc(
			fqName("registries"),
			"Number of Registries",
			[]string{
				"project",
			},
			nil,
		),
		Locations: prometheus.NewDesc(
			fqName("locations"),
			"Number of Locations",
			[]string{
				"project",
				"location",
			},
			nil,
		),
		Formats: prometheus.NewDesc(
			fqName("formats"),
			"Number of Formats",
			[]string{
				"project",
				"format",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *ArtifactRegistryCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	artifactregistryService, err := artifactregistry.NewService(ctx)
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
			log.Printf("[ArtifactRegistryCollector] Project: %s", p.ProjectId)
			name := fmt.Sprintf("projects/%s", p.ProjectId)
			rqst := artifactregistryService.Projects.Locations.List(name)
			resp, err := rqst.Do()
			if err != nil {
				if e, ok := err.(*googleapi.Error); ok {
					if e.Code == http.StatusForbidden {
						// Probably (!) Artifact Registry API has not been enabled for Project (p)
						return
					}

					log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
					return
				}

				log.Println(err)
				return
			}

			repositories := 0
			locations := make(map[string]int)
			formats := make(map[string]int)

			// For each Location
			// Enumerate the list of repositories
			for _, l := range resp.Locations {
				// LocationID is the short form e.g. "us-west1"
				parent := fmt.Sprintf("projects/%s/locations/%s", p.ProjectId, l.LocationId)
				rqst := artifactregistryService.Projects.Locations.Repositories.List(parent)

				for {
					resp, err := rqst.Do()
					if err != nil {
						if e, ok := err.(*googleapi.Error); ok {
							if e.Code == http.StatusForbidden {
								// Probably (!) Cloud Functions API has not been enabled for Project (p)
								return
							}
							log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
						}
						log.Println(err)
						return
					}

					// If there are any repositories in this location
					if len(resp.Repositories) > 0 {
						repositories += len(resp.Repositories)
						locations[l.LocationId]++

						for _, repository := range resp.Repositories {
							formats[repository.Format]++
						}
					}

					// If there are no more pages, we're done
					if resp.NextPageToken == "" {
						break
					}

					// Otherwise, next page
					rqst = rqst.PageToken(resp.NextPageToken)
				}
			}

			ch <- prometheus.MustNewConstMetric(
				c.Registries,
				prometheus.GaugeValue,
				float64(repositories),
				[]string{
					p.ProjectId,
				}...,
			)

			for location, count := range locations {
				ch <- prometheus.MustNewConstMetric(
					c.Locations,
					prometheus.GaugeValue,
					float64(count),
					[]string{
						p.ProjectId,
						location,
					}...,
				)
			}
			for format, count := range formats {
				ch <- prometheus.MustNewConstMetric(
					c.Formats,
					prometheus.GaugeValue,
					float64(count),
					[]string{
						p.ProjectId,
						format,
					}...,
				)
			}
		}(p)
	}
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *ArtifactRegistryCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Registries
	ch <- c.Locations
	ch <- c.Formats
}

package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudfunctions/v1"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
)

var (
	_ prometheus.Collector = (*FunctionsCollector)(nil)
)

// FunctionsCollector represents Cloud Functions
type FunctionsCollector struct {
	account *gcp.Account

	Functions *prometheus.Desc
	Locations *prometheus.Desc
	Runtimes  *prometheus.Desc
}

// NewFunctionsCollector returns a new FunctionsCollector
func NewFunctionsCollector(account *gcp.Account) *FunctionsCollector {
	fqName := name("cloudfunctions")
	return &FunctionsCollector{
		account: account,

		Functions: prometheus.NewDesc(
			fqName("functions"),
			"Number of Cloud Functions",
			[]string{
				"project",
			},
			nil,
		),
		Locations: prometheus.NewDesc(
			fqName("locations"),
			"Number of Functions by Location",
			[]string{
				"project",
				"location",
			},
			nil,
		),
		Runtimes: prometheus.NewDesc(
			fqName("runtimes"),
			"Number of Functions by Runtime",
			[]string{
				"project",
				"runtime",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *FunctionsCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	cloudfunctionsservice, err := cloudfunctions.NewService(ctx)
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
			log.Printf("[CloudFunctionsCollector] Projects: %s", p.ProjectId)
			parent := fmt.Sprintf("projects/%s/locations/-", p.ProjectId)
			rqst := cloudfunctionsservice.Projects.Locations.Functions.List(parent)

			functions := 0
			locations := make(map[string]int)
			runtimes := make(map[string]int)

			// Do request at least once
			for {
				resp, err := rqst.Do()
				if err != nil {
					if e, ok := err.(*googleapi.Error); ok {
						if e.Code == http.StatusForbidden {
							// Probably (!) Cloud Functions API has not been enabled for Project (p)
							return
						}
						log.Printf("Google API Error: %d [%s}", e.Code, e.Message)
					}
					log.Println(err)
					return
				}

				functions += len(resp.Functions)

				// https://cloud.google.com/functions/docs/reference/rest/v1/projects.locations.functions#CloudFunction
				for _, function := range resp.Functions {
					// Name == projects/*/locations/*/functions/*
					log.Printf("[CloudFunctionsCollector] function: %s", function.Name)
					parts := strings.Split(function.Name, "/")
					// 0="projects",1="{project}",2="locations",3="{location}",4="functions",5="{function}"
					if len(parts) != 6 {
						log.Printf("[CloudFunctionsCollector] Unable to parse function name: %s", function.Name)
					}
					// Increment locations count by this function's location
					locations[parts[3]]++

					log.Printf("[CloudFunctionsCollector] runtime: %s", function.Runtime)
					// Increment runtimes count by this function's runtime
					runtimes[function.Runtime]++
				}

				// If there are no more pages, we're done
				if resp.NextPageToken == "" {
					break
				}

				// Otherwise, next page
				rqst = rqst.PageToken(resp.NextPageToken)
			}

			// Now we know the number of Functions
			// Because this count is by project, include project labels to avoid duplication
			// Can always total by location across projects
			// gcp_cloudfunctions_locations{location="us-central1",project="gcp"} 1
			// gcp_cloudfunctions_locations{location="us-central1",project="yyy"} 1
			ch <- prometheus.MustNewConstMetric(
				c.Functions,
				prometheus.GaugeValue,
				float64(functions),
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
			// Can always total by runtime across projects
			// gcp_cloudfunctions_runtimes{project="gcp",runtime="go113"} 1
			// gcp_cloudfunctions_runtimes{project="yyy",runtime="go113"} 1
			for runtime, count := range runtimes {
				ch <- prometheus.MustNewConstMetric(
					c.Runtimes,
					prometheus.GaugeValue,
					float64(count),
					[]string{
						p.ProjectId,
						runtime,
					}...,
				)
			}
		}(p)
	}
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *FunctionsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Functions
}

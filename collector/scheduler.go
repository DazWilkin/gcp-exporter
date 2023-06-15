package collector

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	cloudscheduler "google.golang.org/api/cloudscheduler/v1"
)

var (
	_ prometheus.Collector = (*SchedulerCollector)(nil)
)

// SchedulerCollector represents Cloud Scheduler
type SchedulerCollector struct {
	account *gcp.Account

	Jobs *prometheus.Desc
}

// NewSchedulerCollector returns a new SchedulerCollector
func NewSchedulerCollector(account *gcp.Account) *SchedulerCollector {
	fqName := name("cloud_scheduler")
	return &SchedulerCollector{
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
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *SchedulerCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	schedulerService, err := cloudscheduler.NewService(ctx)
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
			log.Printf("[SchedulerCollector] Project: %s", p.ProjectId)

			name := fmt.Sprintf("projects/%s", p.ProjectId)
			count := 0

			rqst := schedulerService.Projects.Locations.List(name)
			if err := rqst.Pages(ctx, func(page *cloudscheduler.ListLocationsResponse) error {
				for _, l := range page.Locations {
					log.Printf("[SchedulerCollector] Project: %s (Location: %s)", p.ProjectId, l.LocationId)

					name2 := fmt.Sprintf("%s/locations/%s", name, l.LocationId)
					rqst2 := schedulerService.Projects.Locations.Jobs.List(name2)
					if err := rqst2.Pages(ctx, func(page2 *cloudscheduler.ListJobsResponse) error {
						// Count the number of Jobs
						count += len(page2.Jobs)
						// for _, j := range page2.Jobs {
						// 	log.Printf("[SchedulerCollector] Job: %s", j)
						// }
						return nil
					}); err != nil {
						log.Println(err)
						return nil
					}
				}
				return nil
			}); err != nil {
				log.Println(err)
				return
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
		wg.Wait()
	}
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *SchedulerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Jobs
}

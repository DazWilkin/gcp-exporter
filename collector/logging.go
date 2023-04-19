package collector

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/logging/v2"
)

// LoggingCollector represents Cloud Logging
type LoggingCollector struct {
	account *gcp.Account

	Logs *prometheus.Desc
}

// NewLoggingCollector creates a new LoggingCollector
func NewLoggingCollector(account *gcp.Account) *LoggingCollector {
	fqName := name("cloud_logging")

	return &LoggingCollector{
		account: account,

		Logs: prometheus.NewDesc(
			fqName("logs"),
			"Number of Logs",
			[]string{
				"project",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *LoggingCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	loggingService, err := logging.NewService(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	// Enumerate all projects
	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		log.Printf("[LoggingCollector] Project: %s", p.ProjectId)

		name := fmt.Sprintf("projects/%s", p.ProjectId)

		// Logs
		wg.Add(1)
		go func(project string) {
			defer wg.Done()

			count := 0
			rqst := loggingService.Projects.Logs.List(name)
			if err := rqst.Pages(ctx, func(page *logging.ListLogsResponse) error {
				count += len(page.LogNames)
				return nil
			}); err != nil {
				log.Println(err)
				return
			}

			if count != 0 {
				ch <- prometheus.MustNewConstMetric(
					c.Logs,
					prometheus.GaugeValue,
					float64(count),
					[]string{
						project,
					}...,
				)
			}
		}(p.ProjectId)
	}

	// Wait for all projects to process
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *LoggingCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Logs
}

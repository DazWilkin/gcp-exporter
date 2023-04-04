package collector

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"

	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/monitoring/v3"
)

// MonitoringCollector represents Cloud Monitoring
type MonitoringCollector struct {
	account *gcp.Account

	AlertPolicies *prometheus.Desc
	UptimeChecks  *prometheus.Desc
}

// NewMonitoringCollector create a new MonitoringCollector
func NewMonitoringCollector(account *gcp.Account) *MonitoringCollector {
	fqName := name("cloud_monitoring")

	return &MonitoringCollector{
		account: account,

		AlertPolicies: prometheus.NewDesc(
			fqName("alert_policies"),
			"Number of Alert Policies",
			[]string{
				"project",
			},
			nil,
		),
		UptimeChecks: prometheus.NewDesc(
			fqName("uptime_checks"),
			"Number of Uptime Checks",
			[]string{
				"project",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *MonitoringCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	monitoringService, err := monitoring.NewService(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	// Enumerate all projects
	// WaitGroup is used for project AlertPolicies|UptimeChecks
	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		log.Printf("[MonitoringCollector] Project: %s", p.ProjectId)

		name := fmt.Sprintf("projects/%s", p.ProjectId)

		// Alert Policies
		wg.Add(1)
		go func(project string) {
			defer wg.Done()

			count := 0

			rqst := monitoringService.Projects.AlertPolicies.List(name)
			if err := rqst.Pages(ctx, func(page *monitoring.ListAlertPoliciesResponse) error {
				count += len(page.AlertPolicies)
				return nil
			}); err != nil {
				log.Println(err)
				return
			}

			if count != 0 {
				ch <- prometheus.MustNewConstMetric(
					c.AlertPolicies,
					prometheus.GaugeValue,
					float64(count),
					[]string{
						project,
					}...,
				)
			}
		}(p.ProjectId)

		// Uptime Checks
		wg.Add(1)
		go func(project string) {
			defer wg.Done()

			count := 0

			rqst := monitoringService.Projects.UptimeCheckConfigs.List(name)
			if err := rqst.Pages(ctx, func(page *monitoring.ListUptimeCheckConfigsResponse) error {
				count += len(page.UptimeCheckConfigs)
				return nil
			}); err != nil {
				log.Println(err)
				return
			}

			if count != 0 {
				ch <- prometheus.MustNewConstMetric(
					c.UptimeChecks,
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
func (c *MonitoringCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.AlertPolicies
	ch <- c.UptimeChecks
}

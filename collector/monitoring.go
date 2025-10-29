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
	account           *gcp.Account
	monitoringService *monitoring.Service

	AlertPolicies *prometheus.Desc
	Alerts        *prometheus.Desc
	UptimeChecks  *prometheus.Desc
}

// NewMonitoringCollector create a new MonitoringCollector
func NewMonitoringCollector(account *gcp.Account) (*MonitoringCollector, error) {
	subsystem := "cloud_monitoring"

	ctx := context.Background()
	monitoringService, err := monitoring.NewService(ctx)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &MonitoringCollector{
		account:           account,
		monitoringService: monitoringService,

		AlertPolicies: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "alert_policies"),
			"Number of Alert Policies",
			[]string{
				"project",
			},
			nil,
		),
		Alerts: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "alerts"),
			"Number of Alerts",
			[]string{
				"project",
			},
			nil,
		),
		UptimeChecks: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "uptime_checks"),
			"Number of Uptime Checks",
			[]string{
				"project",
			},
			nil,
		),
	}, nil
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *MonitoringCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	// Enumerate all projects
	// WaitGroup is used for project AlertPolicies|UptimeChecks
	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		log.Printf("[MonitoringCollector] Project: %s", p.ProjectId)

		parent := fmt.Sprintf("projects/%s", p.ProjectId)

		c.collectAlertPolicies(ctx, &wg, ch, parent, p.ProjectId)
		c.collectAlerts(ctx, &wg, ch, parent, p.ProjectId)
		c.collectUptimeChecks(ctx, &wg, ch, parent, p.ProjectId)
	}
	// Wait for all projects to process
	wg.Wait()
}

// collectAlertPolicies collects alert policy metrics
func (c *MonitoringCollector) collectAlertPolicies(ctx context.Context, wg *sync.WaitGroup, ch chan<- prometheus.Metric, parent, projectID string) {
	wg.Add(1)
	go func(project string) {
		defer wg.Done()

		count := 0

		rqst := c.monitoringService.Projects.AlertPolicies.List(parent)
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
	}(projectID)
}

// collectAlerts collects alert metrics
func (c *MonitoringCollector) collectAlerts(ctx context.Context, wg *sync.WaitGroup, ch chan<- prometheus.Metric, parent, projectID string) {
	wg.Add(1)
	go func(project string) {
		defer wg.Done()

		count := 0

		rqst := c.monitoringService.Projects.Alerts.List(parent)
		if err := rqst.Pages(ctx, func(page *monitoring.ListAlertsResponse) error {
			count += len(page.Alerts)
			return nil
		}); err != nil {
			log.Println(err)
			return
		}

		if count != 0 {
			ch <- prometheus.MustNewConstMetric(
				c.Alerts,
				prometheus.GaugeValue,
				float64(count),
				[]string{
					project,
				}...,
			)
		}
	}(projectID)
}

// collectUptimeChecks collects uptime check metrics
func (c *MonitoringCollector) collectUptimeChecks(ctx context.Context, wg *sync.WaitGroup, ch chan<- prometheus.Metric, parent, projectID string) {
	wg.Add(1)
	go func(project string) {
		defer wg.Done()

		count := 0

		rqst := c.monitoringService.Projects.UptimeCheckConfigs.List(parent)
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
	}(projectID)
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *MonitoringCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.AlertPolicies
	ch <- c.UptimeChecks
}

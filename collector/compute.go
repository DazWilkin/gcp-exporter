package collector

import (
	"context"
	"log"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

// ComputeCollector represents Compute Engine
type ComputeCollector struct {
	account *gcp.Account

	Instances       *prometheus.Desc
	ForwardingRules *prometheus.Desc
}

// NewComputeCollector returns a new ComputeCollector
func NewComputeCollector(account *gcp.Account) *ComputeCollector {
	fqName := name("compute_engine")
	return &ComputeCollector{
		account: account,

		Instances: prometheus.NewDesc(
			fqName("instances"),
			"Number of instances",
			[]string{
				"project",
				"zone",
			},
			nil,
		),
		ForwardingRules: prometheus.NewDesc(
			fqName("forwardingrules"),
			"Number of forwardingrules",
			[]string{
				"project",
				"region",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *ComputeCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
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
			log.Printf("[ComputeCollector] Project: %s", p.ProjectId)

			wg.Add(1)
			go func(p *cloudresourcemanager.Project) {
				defer wg.Done()
				// Compute Engine API instances.list requires zone
				// Must repeat the call for all possible zones
				zoneList, err := computeService.Zones.List(p.ProjectId).Context(ctx).Do()
				if err != nil {
					if e, ok := err.(*googleapi.Error); ok {
						log.Printf("[ComputeCollector] Project: %s -- Zones.List (%d)", p.ProjectId, e.Code)
					}
					return
				}
				for _, z := range zoneList.Items {
					wg.Add(1)
					go func(z *compute.Zone) {
						defer wg.Done()
						rqst := computeService.Instances.List(p.ProjectId, z.Name).MaxResults(500)
						count := 0
						// Page through more results
						if err := rqst.Pages(ctx, func(page *compute.InstanceList) error {
							count += len(page.Items)
							// for _, instance := range page.Items {
							// 	instance.
							// }
							return nil
						}); err != nil {
							log.Println(err)
							return
						}
						if count != 0 {
							ch <- prometheus.MustNewConstMetric(
								c.Instances,
								prometheus.GaugeValue,
								float64(count),
								[]string{
									p.ProjectId,
									z.Name,
								}...,
							)
						}
					}(z)
				}
			}(p)

			wg.Add(1)
			go func(p *cloudresourcemanager.Project) {
				defer wg.Done()
				// Compute Engine API forwardingrules.list requires region
				// Must repeat call for all possible regions
				regionList, err := computeService.Regions.List(p.ProjectId).Context(ctx).Do()
				if err != nil {
					if e, ok := err.(*googleapi.Error); ok {
						log.Printf("[ComputeCollector] Project: %s -- Regions.List (%d)", p.ProjectId, e.Code)
					} else {
						log.Println(err)
					}
					return
				}
				for _, r := range regionList.Items {
					wg.Add(1)
					go func(r *compute.Region) {
						defer wg.Done()
						rqst := computeService.ForwardingRules.List(p.ProjectId, r.Name).MaxResults(500)
						count := 0
						if err := rqst.Pages(ctx, func(page *compute.ForwardingRuleList) error {
							count += len(page.Items)
							return nil
						}); err != nil {
							log.Println(err)
							return
						}
						if count != 0 {
							ch <- prometheus.MustNewConstMetric(
								c.ForwardingRules,
								prometheus.GaugeValue,
								float64(count),
								[]string{
									p.ProjectId,
									r.Name,
								}...,
							)
						}
					}(r)
				}
			}(p)
		}(p)
	}
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *ComputeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Instances
	ch <- c.ForwardingRules
}

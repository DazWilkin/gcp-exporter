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
	account        *gcp.Account
	computeService *compute.Service

	Instances       *prometheus.Desc
	ForwardingRules *prometheus.Desc
}

// NewComputeCollector returns a new ComputeCollector
func NewComputeCollector(account *gcp.Account) (*ComputeCollector, error) {
	subsystem := "compute_engine"

	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &ComputeCollector{
		account:        account,
		computeService: computeService,

		Instances: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "instances"),
			"Number of instances",
			[]string{
				"project",
				"zone",
			},
			nil,
		),
		ForwardingRules: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "forwardingrules"),
			"Number of forwardingrules",
			[]string{
				"project",
				"region",
			},
			nil,
		),
	}, nil
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *ComputeCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	// Enumerate all of the projects
	// WaitGroup is used for project Instances|ForwardingRules only (not the projects themselves)
	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		log.Printf("[ComputeCollector] Project: %s", p.ProjectId)

		wg.Add(1)
		go func(p *cloudresourcemanager.Project) {
			defer wg.Done()
			// Compute Engine API instances.list requires zone
			// Must repeat the call for all possible zones
			zoneList, err := c.computeService.Zones.List(p.ProjectId).Context(ctx).Do()
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
					rqst := c.computeService.Instances.List(p.ProjectId, z.Name).MaxResults(500)
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
			regionList, err := c.computeService.Regions.List(p.ProjectId).Context(ctx).Do()
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
					rqst := c.computeService.ForwardingRules.List(p.ProjectId, r.Name).MaxResults(500)
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
	}
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *ComputeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Instances
	ch <- c.ForwardingRules
}

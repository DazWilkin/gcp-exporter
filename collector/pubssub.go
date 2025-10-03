package collector

import (
	"context"
	"fmt"
	"log"
	"path"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/pubsub/v1"
)

type PubSubCollector struct {
	account       *gcp.Account
	pubsubService *pubsub.Service

	Schemas       *prometheus.Desc
	Snapshots     *prometheus.Desc
	Subscriptions *prometheus.Desc
	Topics        *prometheus.Desc
	// Up            *prometheus.Desc
}

func NewPubSubCollector(account *gcp.Account, endpoint string) (*PubSubCollector, error) {
	subsystem := "pubsub"

	ctx := context.Background()

	opts := []option.ClientOption{}
	if endpoint != "" {
		opts = append(opts, option.WithEndpoint(endpoint))
	}

	pubsubService, err := pubsub.NewService(ctx, opts...)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &PubSubCollector{
		account:       account,
		pubsubService: pubsubService,

		// https://pkg.go.dev/google.golang.org/api@v0.242.0/pubsub/v1#Schema
		Schemas: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "schemas"),
			"Number of schemas",
			[]string{
				"project",
				"name",
				"type",
			},
			nil,
		),
		// https://pkg.go.dev/google.golang.org/api@v0.242.0/pubsub/v1#Snapshot
		Snapshots: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "snapshots"),
			"Number of Snapshots",
			[]string{
				"project",
				"name",
				"topic",
			},
			nil,
		),
		// https://pkg.go.dev/google.golang.org/api@v0.242.0/pubsub/v1#Subscription
		Subscriptions: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "subscriptions"),
			"Number of subscriptions",
			[]string{
				"project",
				"name",
				"state",
				"topic",
			},
			nil,
		),
		// https://pkg.go.dev/google.golang.org/api@v0.242.0/pubsub/v1#Topic
		Topics: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "topics"),
			"Number of topics",
			[]string{
				"project",
				"name",
				"state",
			},
			nil,
		),
		// Up: prometheus.NewDesc(
		// 	prometheus.BuildFQName(prefix, subsystem, "up"),
		// 	"1 if the topic is accessible, 0 otherwise",
		// 	[]string{},
		// 	nil,
		// ),
	}, nil
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *PubSubCollector) Collect(ch chan<- prometheus.Metric) {
	// ctx := context.Background()

	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		log.Printf("[PubSubCollector] Project: %s", p.ProjectId)

		// Schemas
		wg.Add(1)
		go c.collectSchemas(&wg, ch, p)

		// Snapshots
		wg.Add(1)
		go c.collectSnapshots(&wg, ch, p)

		// Subscriptions
		wg.Add(1)
		go c.collectSubscriptions(&wg, ch, p)

		// Topics
		wg.Add(1)
		go c.collectTopics(&wg, ch, p)
	}
	wg.Wait()
}

// collectSchemas collects schema metrics for a project
func (c *PubSubCollector) collectSchemas(wg *sync.WaitGroup, ch chan<- prometheus.Metric, p *cloudresourcemanager.Project) {
	defer wg.Done()

	project := fmt.Sprintf("projects/%s", p.ProjectId)
	rqst := c.pubsubService.Projects.Schemas.List(project)
	resp, err := rqst.Do()
	if err != nil {
		log.Printf("[PubSubCollector] Error listing schemas for %s: %v", p.ProjectId, err)
		return
	}

	for _, s := range resp.Schemas {
		ch <- prometheus.MustNewConstMetric(
			c.Schemas,
			prometheus.GaugeValue,
			1,
			[]string{
				p.ProjectId,
				// https://pkg.go.dev/path#Base
				path.Base(s.Name),
				s.Type,
			}...,
		)
	}
}

// collectSnapshots collects snapshot metrics for a project
func (c *PubSubCollector) collectSnapshots(wg *sync.WaitGroup, ch chan<- prometheus.Metric, p *cloudresourcemanager.Project) {
	defer wg.Done()

	project := fmt.Sprintf("projects/%s", p.ProjectId)
	rqst := c.pubsubService.Projects.Snapshots.List(project)
	resp, err := rqst.Do()
	if err != nil {
		log.Printf("[PubSubCollector] Error listing snapshots for %s: %v", p.ProjectId, err)
		return
	}

	for _, s := range resp.Snapshots {
		ch <- prometheus.MustNewConstMetric(
			c.Snapshots,
			prometheus.GaugeValue,
			1,
			[]string{
				p.ProjectId,
				// https://pkg.go.dev/path#Base
				path.Base(s.Name),
				path.Base(s.Topic),
			}...,
		)
	}
}

// collectSubscriptions collects subscription metrics for a project
func (c *PubSubCollector) collectSubscriptions(wg *sync.WaitGroup, ch chan<- prometheus.Metric, p *cloudresourcemanager.Project) {
	defer wg.Done()

	project := fmt.Sprintf("projects/%s", p.ProjectId)
	rqst := c.pubsubService.Projects.Subscriptions.List(project)
	resp, err := rqst.Do()
	if err != nil {
		log.Printf("[PubSubCollector] Error listing subscriptions for %s: %v", p.ProjectId, err)
		return
	}

	for _, s := range resp.Subscriptions {
		ch <- prometheus.MustNewConstMetric(
			c.Subscriptions,
			prometheus.GaugeValue,
			1,
			[]string{
				p.ProjectId,
				// https://pkg.go.dev/path#Base
				path.Base(s.Name),
				s.State,
				path.Base(s.Topic),
			}...,
		)
	}
}

// collectTopics collects topic metrics for a project
func (c *PubSubCollector) collectTopics(wg *sync.WaitGroup, ch chan<- prometheus.Metric, p *cloudresourcemanager.Project) {
	defer wg.Done()

	project := fmt.Sprintf("projects/%s", p.ProjectId)
	rqst := c.pubsubService.Projects.Topics.List(project)
	resp, err := rqst.Do()
	if err != nil {
		log.Printf("[PubSubCollector] Error listing topics for %s: %v", p.ProjectId, err)
		return
	}

	for _, t := range resp.Topics {
		ch <- prometheus.MustNewConstMetric(
			c.Topics,
			prometheus.GaugeValue,
			1,
			[]string{
				p.ProjectId,
				// https://pkg.go.dev/path#Base
				path.Base(t.Name),
				t.State,
			}...,
		)
	}
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *PubSubCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Schemas
	ch <- c.Snapshots
	ch <- c.Subscriptions
	ch <- c.Topics
	// ch <- c.Up
}

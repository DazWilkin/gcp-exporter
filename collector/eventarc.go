package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/eventarc/v1"
	"google.golang.org/api/googleapi"
)

// EventarcCollector represents EventArc
type EventarcCollector struct {
	account *gcp.Account

	Channels *prometheus.Desc
	Triggers *prometheus.Desc
}

// NewEventarcCollector creates a new EventarcCollector
func NewEventarcCollector(account *gcp.Account) *EventarcCollector {
	fqName := name("eventarc")
	return &EventarcCollector{
		account: account,

		Channels: prometheus.NewDesc(
			fqName("channels"),
			"1 if the channel exists",
			[]string{
				"name",
				"provider",
				"pubsubtopic",
				"state",
			},
			nil,
		),
		Triggers: prometheus.NewDesc(
			fqName("triggers"),
			"1 if the trigger exists",
			[]string{
				"name",
				"channel",
				"contenttype",
				"destination",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *EventarcCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	eventarcService, err := eventarc.NewService(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	// Enumerate all of the projects
	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		log.Printf("[EventarcCollector] Project: %s", p.ProjectId)
		parent := fmt.Sprintf("projects/%s/locations/-", p.ProjectId)

		// Channels
		wg.Add(1)
		go func() {
			defer wg.Done()

			rqst := eventarcService.Projects.Locations.Channels.List(parent)
			resp, err := rqst.Do()
			if err != nil {
				if e, ok := err.(*googleapi.Error); ok {
					if e.Code == http.StatusForbidden {
						// Probably (!) Eventarc API has not enabled in this Project
						return
					}

					log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
					return
				}

				log.Println(err)
				return
			}

			for _, channel := range resp.Channels {
				log.Printf("[EventarcCollector] channel: %s", channel.Name)
				ch <- prometheus.MustNewConstMetric(
					c.Channels,
					prometheus.CounterValue,
					1.0,
					[]string{
						channel.Name,
						channel.Provider,
						channel.PubsubTopic,
						channel.State,
					}...,
				)
			}
		}()

		// Triggers
		wg.Add(1)
		go func() {
			defer wg.Done()

			rqst := eventarcService.Projects.Locations.Triggers.List(parent)
			resp, err := rqst.Do()
			if err != nil {
				if e, ok := err.(*googleapi.Error); ok {
					if e.Code == http.StatusForbidden {
						// Probably (!) Eventarc API has not enabled in this Project
						return
					}

					log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
					return
				}

				log.Println(err)
				return
			}

			for _, trigger := range resp.Triggers {
				log.Printf("[EventarcCollector] trigger: %s", trigger.Name)
				ch <- prometheus.MustNewConstMetric(
					c.Triggers,
					prometheus.CounterValue,
					1.0,
					[]string{
						trigger.Name,
						trigger.Channel,
						trigger.EventDataContentType,
						func(d *eventarc.Destination) string {
							if d.CloudFunction != "" {
								return "cloudfunction"
							}
							if d.CloudRun != nil {
								return "cloudrun"
							}
							if d.Gke != nil {
								return "gke"
							}
							if d.Workflow != "" {
								return "workflow"
							}
							return ""
						}(trigger.Destination),
					}...,
				)
			}
		}()
	}
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *EventarcCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Channels
	ch <- c.Triggers
}

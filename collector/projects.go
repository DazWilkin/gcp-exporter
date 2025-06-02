package collector

import (
	"context"
	"log"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/cloudresourcemanager/v1"
)

// ProjectsCollector represents Google Cloud Platform projects
type ProjectsCollector struct {
	account                     *gcp.Account
	cloudresourcemanagerService *cloudresourcemanager.Service

	filter   string
	pagesize int64

	Count *prometheus.Desc
}

// NewProjectsCollector returns a new ProjectsCollector
func NewProjectsCollector(account *gcp.Account, filter string, pagesize int64) (*ProjectsCollector, error) {
	subsystem := "projects"

	// Combine any user-specified filter with "lifecycleState:ACTIVE" to only process active projects
	if filter != "" {
		filter += " "
	}
	filter = filter + "lifecycleState:ACTIVE"
	log.Printf("Projects filter: '%s'", filter)

	ctx := context.Background()
	cloudresourcemanagerService, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return &ProjectsCollector{
		account:                     account,
		cloudresourcemanagerService: cloudresourcemanagerService,

		filter:   filter,
		pagesize: pagesize,

		Count: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, subsystem, "count"),
			"Number of Projects",
			[]string{},
			nil,
		),
	}, nil
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *ProjectsCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	// Create the Projects.List request
	// Return at most (!) '--pagesize' projects
	// Filter the results to only include the project ID and number
	req := c.cloudresourcemanagerService.Projects.List().PageSize(c.pagesize).Fields("projects.projectId", "projects.projectNumber").Filter(c.filter)

	projects := []*cloudresourcemanager.Project{}

	// Do request at least once
	for {
		resp, err := req.Context(ctx).Do()
		if err != nil {
			log.Println("Unable to list projects")
			return
		}

		if len(resp.Projects) == 0 {
			log.Println("There are 0 projects. Nothing to do")
			return
		}

		// Append projects
		projects = append(projects, resp.Projects...)

		if resp.NextPageToken == "" {
			break
		}

	}

	// Now we have a revised list of projects
	// Update the shard list
	c.account.Update(projects)

	// Update the metric
	ch <- prometheus.MustNewConstMetric(
		c.Count,
		prometheus.GaugeValue,
		float64(len(projects)),
		[]string{}...,
	)

}

// Describe implements Prometheus' Collector interface and is used to desribe metrics
func (c *ProjectsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Count
}

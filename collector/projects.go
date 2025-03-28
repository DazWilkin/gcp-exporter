package collector

import (
	"context"
	"log"
	"strconv"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/cloudresourcemanager/v1"
)

// ProjectsCollector represents Google Cloud Platform projects
type ProjectsCollector struct {
	filter                        string
	pagesize                      int64
	account                       *gcp.Account
	enableExtendedMetrics         bool
	extraLabelsInfoExtendedMetric ExtraLabel

	count *prometheus.Desc
	info  *prometheus.Desc
}

// NewProjectsCollector returns a new ProjectsCollector
func NewProjectsCollector(account *gcp.Account, filter string, pagesize int64, enableExtendedMetric bool, extraLabelsInfo string) *ProjectsCollector {
	fqName := name("projects")

	log.Printf("Projects filter: '%s'", filter)

	extraLabelsInfoMap := ProcessExtraLabels(extraLabelsInfo)

	return &ProjectsCollector{
		filter:                        filter,
		pagesize:                      pagesize,
		account:                       account,
		enableExtendedMetrics:         enableExtendedMetric,
		extraLabelsInfoExtendedMetric: extraLabelsInfoMap,

		count: prometheus.NewDesc(
			fqName("count"),
			"Number of Projects",
			[]string{},
			nil,
		),
		info: prometheus.NewDesc(
			fqName("info"),
			"Info by Project",
			append([]string{"name", "id", "number", "parent_type", "parent_id"},
				GetLabelNamesFromExtraLabels(extraLabelsInfoMap)...),
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *ProjectsCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	cloudresourcemanagerService, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		log.Println("Error initializing Cloud Resource Manager service:", err)
		return
	}

	// Create the Projects.List request with the given filter
	req := cloudresourcemanagerService.Projects.List().
		PageSize(c.pagesize).
		Fields("projects.name", "projects.projectId", "projects.projectNumber", "projects.parent", "projects.labels", "projects.lifecycleState").
		Filter(c.filter)

	// Initialize collections to hold the filtered projects
	var allProjects, activeProjects []*cloudresourcemanager.Project

	// Loop through the paginated results
	for {
		resp, err := req.Context(ctx).Do()
		if err != nil {
			log.Println("Unable to list projects:", err)
			return
		}

		if len(resp.Projects) == 0 {
			log.Println("There are 0 projects. Nothing to do")
			break
		}

		// Filter and append the projects to the appropriate slices
		for _, project := range resp.Projects {
			allProjects = append(allProjects, project)
			if project.LifecycleState == "ACTIVE" {
				activeProjects = append(activeProjects, project)
			}
		}

		// Break if there are no more pages
		if resp.NextPageToken == "" {
			break
		}
	}

	// Update the Prometheus metric for count
	ch <- prometheus.MustNewConstMetric(
		c.count,
		prometheus.GaugeValue,
		float64(len(activeProjects)),
	)

	// Collect extended metrics if enabled
	if c.enableExtendedMetrics {
		c.collectExtendedMetrics(allProjects, ch)
	}

	// Update the shard list with active projects
	c.account.Update(activeProjects)
}

// collectExtendedMetrics processes each project and collects extended metrics
func (c *ProjectsCollector) collectExtendedMetrics(projects []*cloudresourcemanager.Project, ch chan<- prometheus.Metric) {
	for _, project := range projects {
		var parentType, parentId string

		// Retrieve parent information
		if project.Parent != nil {
			parentType = project.Parent.Type
			parentId = project.Parent.Id
		}

		// Determine the lifecycle state for the metric (1.0 if active, else 0.0)
		lifecycleState := 0.0
		if project.LifecycleState == "ACTIVE" {
			lifecycleState = 1.0
		}

		// Prepare the label values
		labelValuesInfo := []string{
			project.Name, project.ProjectId, strconv.FormatInt(project.ProjectNumber, 10), parentType, parentId,
		}

		// Add any extra labels
		labelValuesInfo = append(labelValuesInfo, GetExtraLabelsValues(project.Labels, c.extraLabelsInfoExtendedMetric)...)

		// Send the project info to Prometheus
		ch <- prometheus.MustNewConstMetric(c.info, prometheus.GaugeValue, lifecycleState, labelValuesInfo...)
	}
}

// Describe implements Prometheus' Collector interface to describe the metrics
func (c *ProjectsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.count
	ch <- c.info
}

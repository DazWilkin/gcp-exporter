package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ExporterCollector collects metrics, mostly runtime, about this exporter in general.
type ExporterCollector struct {
	gitCommit string
	goVersion string
	osVersion string
	startTime int64

	StartTime *prometheus.Desc
	BuildInfo *prometheus.Desc
}

// NewExporterCollector returns a new ExporterCollector.
func NewExporterCollector(osVersion, goVersion, gitCommit string, startTime int64) *ExporterCollector {
	fqName := name("exporter")
	return &ExporterCollector{
		osVersion: osVersion,
		goVersion: goVersion,
		gitCommit: gitCommit,
		startTime: startTime,

		StartTime: prometheus.NewDesc(
			fqName("start_time"),
			"start time in Unix epoch seconds",
			nil,
			nil,
		),
		BuildInfo: prometheus.NewDesc(
			fqName("build_info"),
			"A metric with a constant '1' value labeled by OS version, Go version, and the Git commit of the exporter",
			[]string{"os_version", "go_version", "git_commit"},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *ExporterCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		c.StartTime,
		prometheus.GaugeValue,
		float64(c.startTime),
	)
	ch <- prometheus.MustNewConstMetric(
		c.BuildInfo,
		prometheus.CounterValue,
		1.0,
		c.osVersion, c.goVersion, c.gitCommit,
	)
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *ExporterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.StartTime
}

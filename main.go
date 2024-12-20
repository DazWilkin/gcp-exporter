package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/DazWilkin/gcp-exporter/collector"
	"github.com/DazWilkin/gcp-exporter/gcp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// GitCommit is the git commit value and is expected to be set during build
	GitCommit string
	// GoVersion is the Golang runtime version
	GoVersion = runtime.Version()
	// OSVersion is the OS version (uname --kernel-release) and is expected to be set during build
	OSVersion string
	// StartTime is the start time of the exporter represented as a UNIX epoch
	StartTime = time.Now().Unix()
)
var (
	filter      = flag.String("filter", "", "Filter the results of the request")
	pagesize    = flag.Int64("max_projects", 10, "Maximum number of projects to include")
	endpoint    = flag.String("endpoint", ":9402", "The endpoint of the HTTP server")
	metricsPath = flag.String("path", "/metrics", "The path on which Prometheus metrics will be served")
)

const (
	rootTemplate = `<!DOCTYPE html>
	<html>
	<head>
		<title>GCP Exporter</title>
	</head>
	<body>
		<h2>Google Cloud Platform Resources Exporter</h2>
		<ul>
			<li><a href="{{.MetricsPath}}">metrics</a></li>
			<li><a href="/healthz">healthz</a></li>
		</ul>
	<body>
	</html>`
)

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
func handleRoot(w http.ResponseWriter, _ *http.Request) {
	tmpl := template.Must(template.New("root").Parse(rootTemplate))
	data := struct {
		MetricsPath string
	}{
		MetricsPath: *metricsPath, // Assuming metricsPath is a global variable
	}

	if err := tmpl.Execute(w, data); err != nil {
		msg := "error rendering root template"
		log.Printf("[handleRoot] %s: %v", msg, err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
}
func main() {
	flag.Parse()

	if GitCommit == "" {
		log.Println("[main] GitCommit value unchanged: expected to be set during build")
	}
	if OSVersion == "" {
		log.Println("[main] OSVersion value unchanged: expected to be set during build")
	}

	// Objects that holds GCP-specific resources (e.g. projects)
	account := gcp.NewAccount()

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector.NewExporterCollector(OSVersion, GoVersion, GitCommit, StartTime))

	// ProjectCollector is a special case
	// When it runs it replaces the Exporter's list of GCP projects
	// The other collectors are dependent on this list of projects
	registry.MustRegister(collector.NewProjectsCollector(account, *filter, *pagesize))

	registry.MustRegister(collector.NewArtifactRegistryCollector(account))
	registry.MustRegister(collector.NewCloudRunCollector(account))
	registry.MustRegister(collector.NewComputeCollector(account))
	registry.MustRegister(collector.NewEndpointsCollector(account))
	registry.MustRegister(collector.NewEventarcCollector(account))
	registry.MustRegister(collector.NewFunctionsCollector(account))
	registry.MustRegister(collector.NewIAMCollector(account))
	registry.MustRegister(collector.NewKubernetesCollector(account))
	registry.MustRegister(collector.NewLoggingCollector(account))
	registry.MustRegister(collector.NewMonitoringCollector(account))
	registry.MustRegister(collector.NewSchedulerCollector(account))
	registry.MustRegister(collector.NewStorageCollector(account))

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(handleRoot))
	mux.Handle("/healthz", http.HandlerFunc(handleHealthz))
	mux.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	log.Printf("[main] Server starting (%s)", *endpoint)
	log.Printf("[main] metrics served on: %s", *metricsPath)
	log.Fatal(http.ListenAndServe(*endpoint, mux))
}

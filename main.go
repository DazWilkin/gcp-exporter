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
	filter      										  = flag.String("filter", "", "Filter the results of the request")
	pagesize    										  = flag.Int64("max_projects", 10, "Maximum number of projects to include")
	endpoint    										  = flag.String("endpoint", ":9402", "The endpoint of the HTTP server")
	metricsPath 										  = flag.String("path", "/metrics", "The path on which Prometheus metrics will be served")
	DisableArtifactRegistryCollector 	= flag.Bool("collector.artifact_registry.disable", false, "Disables the metrics collector for the Artifact Registry")
	DisableCloudRunCollector          = flag.Bool("collector.cloud_run.disable", false, "Disables the metrics collector for the Cloud Run")
	DisableComputeCollector 				  = flag.Bool("collector.compute.disable", false, "Disables the metrics collector for the Compute")
	DisableEndpointsCollector 				= flag.Bool("collector.endpoints.disable", false, "Disables the metrics collector for the Cloud Endpoints Services")
	DisableEventarcCollector 					= flag.Bool("collector.eventarc.disable", false, "Disables the metrics collector for the Cloud Eventarc")
	DisableFunctionsCollector 				= flag.Bool("collector.functions.disable", false, "Disables the metrics collector for the Cloud Functions")
	DisableIAMCollector 							= flag.Bool("collector.iam.disable", false, "Disables the metrics collector for the Cloud IAM")
	DisableKubernetesCollector 				= flag.Bool("collector.kubernetes.disable", false, "Disables the metrics collector for the Kubernetes (GKE)")
	DisableLoggingCollector 					= flag.Bool("collector.logging.disable", false, "Disables the metrics collector for the Cloud Logging")
	DisableMonitoringCollector 				= flag.Bool("collector.monitoring.disable", false, "Disables the metrics collector for the Cloud Monitoring")
	DisableSchedulerCollector 				= flag.Bool("collector.scheduler.disable", false, "Disables the metrics collector for the Cloud Scheduler")
	DisableStorageCollector 					= flag.Bool("collector.storage.disable", false, "Disables the metrics collector for the Cloud Storage")
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

	if !*DisableArtifactRegistryCollector {
		registry.MustRegister(collector.NewArtifactRegistryCollector(account))
	}

	if !*DisableCloudRunCollector {
		registry.MustRegister(collector.NewCloudRunCollector(account))
	}

	if !*DisableComputeCollector {
		registry.MustRegister(collector.NewComputeCollector(account))
	}

	if !*DisableEndpointsCollector {
		registry.MustRegister(collector.NewEndpointsCollector(account))
	}

	if !*DisableEventarcCollector {
		registry.MustRegister(collector.NewEventarcCollector(account))
	}

	if !*DisableFunctionsCollector {
		registry.MustRegister(collector.NewFunctionsCollector(account))
	}

	if !*DisableIAMCollector {
		registry.MustRegister(collector.NewIAMCollector(account))
	}

	if !*DisableKubernetesCollector {
		registry.MustRegister(collector.NewKubernetesCollector(account))
	}

	if !*DisableLoggingCollector {
		registry.MustRegister(collector.NewLoggingCollector(account))
	}

	if !*DisableMonitoringCollector {
		registry.MustRegister(collector.NewMonitoringCollector(account))
	}

	if !*DisableSchedulerCollector {
		registry.MustRegister(collector.NewSchedulerCollector(account))
	}

	if !*DisableStorageCollector {
		registry.MustRegister(collector.NewStorageCollector(account))
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(handleRoot))
	mux.Handle("/healthz", http.HandlerFunc(handleHealthz))
	mux.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	log.Printf("[main] Server starting (%s)", *endpoint)
	log.Printf("[main] metrics served on: %s", *metricsPath)
	log.Fatal(http.ListenAndServe(*endpoint, mux))
}

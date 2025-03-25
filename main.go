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

	disableArtifactRegistryCollector = flag.Bool("collector.artifact_registry.disable", false, "Disables the metrics collector for the Artifact Registry")
	disableCloudRunCollector         = flag.Bool("collector.cloud_run.disable", false, "Disables the metrics collector for Cloud Run")
	disableComputeCollector          = flag.Bool("collector.compute.disable", false, "Disables the metrics collector for Compute Engine")
	disableEndpointsCollector        = flag.Bool("collector.endpoints.disable", false, "Disables the metrics collector for Cloud Endpoints")
	disableEventarcCollector         = flag.Bool("collector.eventarc.disable", false, "Disables the metrics collector for Cloud Eventarc")
	disableFunctionsCollector        = flag.Bool("collector.functions.disable", false, "Disables the metrics collector for Cloud Functions")
	disableIAMCollector              = flag.Bool("collector.iam.disable", false, "Disables the metrics collector for Cloud IAM")
	disableKubernetesCollector       = flag.Bool("collector.kubernetes.disable", false, "Disables the metrics collector for GKE")
	disableLoggingCollector          = flag.Bool("collector.logging.disable", false, "Disables the metrics collector for Cloud Logging")
	disableMonitoringCollector       = flag.Bool("collector.monitoring.disable", false, "Disables the metrics collector for Cloud Monitoring")
	disableSchedulerCollector        = flag.Bool("collector.scheduler.disable", false, "Disables the metrics collector for Cloud Scheduler")
	disableStorageCollector          = flag.Bool("collector.storage.disable", false, "Disables the metrics collector for Cloud Storage")
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

	collectorConfigs := map[string]struct {
		register func(account *gcp.Account) prometheus.Collector
		disable  *bool
	}{
		"artifact_registry": {func(account *gcp.Account) prometheus.Collector { return collector.NewArtifactRegistryCollector(account) }, disableArtifactRegistryCollector},
		"cloud_run":         {func(account *gcp.Account) prometheus.Collector { return collector.NewCloudRunCollector(account) }, disableCloudRunCollector},
		"compute":           {func(account *gcp.Account) prometheus.Collector { return collector.NewComputeCollector(account) }, disableComputeCollector},
		"endpoints":         {func(account *gcp.Account) prometheus.Collector { return collector.NewEndpointsCollector(account) }, disableEndpointsCollector},
		"eventarc":          {func(account *gcp.Account) prometheus.Collector { return collector.NewEventarcCollector(account) }, disableEventarcCollector},
		"functions":         {func(account *gcp.Account) prometheus.Collector { return collector.NewFunctionsCollector(account) }, disableFunctionsCollector},
		"iam":               {func(account *gcp.Account) prometheus.Collector { return collector.NewIAMCollector(account) }, disableIAMCollector},
		"kubernetes":        {func(account *gcp.Account) prometheus.Collector { return collector.NewKubernetesCollector(account) }, disableKubernetesCollector},
		"logging":           {func(account *gcp.Account) prometheus.Collector { return collector.NewLoggingCollector(account) }, disableLoggingCollector},
		"monitoring":        {func(account *gcp.Account) prometheus.Collector { return collector.NewMonitoringCollector(account) }, disableMonitoringCollector},
		"scheduler":         {func(account *gcp.Account) prometheus.Collector { return collector.NewSchedulerCollector(account) }, disableSchedulerCollector},
		"storage":           {func(account *gcp.Account) prometheus.Collector { return collector.NewStorageCollector(account) }, disableStorageCollector},
	}

	for name, config := range collectorConfigs {
		if config.disable != nil && !*config.disable {
			log.Printf("Registering collector: %s", name)
			registry.MustRegister(config.register(account))
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(handleRoot))
	mux.Handle("/healthz", http.HandlerFunc(handleHealthz))
	mux.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	log.Printf("[main] Server starting (%s)", *endpoint)
	log.Printf("[main] metrics served on: %s", *metricsPath)
	log.Fatal(http.ListenAndServe(*endpoint, mux))
}

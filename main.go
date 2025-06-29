package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	_ "net/http/pprof"
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

	profilingEnabled  = flag.Bool("profiling_enabled", false, "Enable profiling endpoint")
	profilingEndpoint = flag.String("profiling_endpoint", ":6060", "The endpoint of the profiling server")

	disableArtifactRegistryCollector = flag.Bool("collector.artifact_registry.disable", false, "Disables the metrics collector for the Artifact Registry")
	disableCloudRunCollector         = flag.Bool("collector.cloud_run.disable", false, "Disables the metrics collector for Cloud Run")
	disableComputeCollector          = flag.Bool("collector.compute.disable", false, "Disables the metrics collector for Compute Engine")
	disableEndpointsCollector        = flag.Bool("collector.endpoints.disable", false, "Disables the metrics collector for Cloud Endpoints")
	disableEventarcCollector         = flag.Bool("collector.eventarc.disable", false, "Disables the metrics collector for Cloud Eventarc")
	disableFunctionsCollector        = flag.Bool("collector.functions.disable", false, "Disables the metrics collector for Cloud Functions")
	disableIAMCollector              = flag.Bool("collector.iam.disable", false, "Disables the metrics collector for Cloud IAM")
	disableGKECollector              = flag.Bool("collector.gke.disable", false, "Disables the metrics collector for Google Kubernetes Engine (GKE)")
	disableLoggingCollector          = flag.Bool("collector.logging.disable", false, "Disables the metrics collector for Cloud Logging")
	disableMonitoringCollector       = flag.Bool("collector.monitoring.disable", false, "Disables the metrics collector for Cloud Monitoring")
	disableSchedulerCollector        = flag.Bool("collector.scheduler.disable", false, "Disables the metrics collector for Cloud Scheduler")
	disableStorageCollector          = flag.Bool("collector.storage.disable", false, "Disables the metrics collector for Cloud Storage")

	enableExtendedMetricsGKECollector = flag.Bool("collector.gke.extendedMetrics.enable", false, "Enable the metrics collector for Google Kubernetes Engine (GKE) to collect ControlPlane and NodePool metrics")
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
	if _, err := w.Write([]byte("ok")); err != nil {
		msg := "error writing healthz handler"
		log.Printf("[handleHealthz] %s: %v", msg, err)
	}
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

func must[T prometheus.Collector](collector T, err error) T {
	if err != nil {
		log.Fatal(err)
	}

	return collector
}

func main() {
	flag.Parse()

	if *disableGKECollector && *enableExtendedMetricsGKECollector {
		log.Println("[main] `--enabledExtendedMetricsGKECollector` has no effect because `--disableGKECollector=true`")
	}

	if GitCommit == "" {
		log.Println("[main] GitCommit value unchanged: expected to be set during build")
	}
	if OSVersion == "" {
		log.Println("[main] OSVersion value unchanged: expected to be set during build")
	}

	// Profiling
	if *profilingEnabled {
		go func() {
			log.Printf("[main] Profiling server starting (%s)", *profilingEndpoint)
			log.Fatal(http.ListenAndServe(*profilingEndpoint, nil))
		}()
	}

	// Objects that holds GCP-specific resources (e.g. projects)
	account := gcp.NewAccount()

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector.NewExporterCollector(OSVersion, GoVersion, GitCommit, StartTime))

	// ProjectCollector is a special case
	// When it runs it replaces the Exporter's list of GCP projects
	// The other collectors are dependent on this list of projects
	registry.MustRegister(must(collector.NewProjectsCollector(account, *filter, *pagesize)))

	collectorConfigs := map[string]struct {
		collector prometheus.Collector
		disable   *bool
	}{
		"artifact_registry": {
			must(collector.NewArtifactRegistryCollector(account)),
			disableArtifactRegistryCollector,
		},
		"cloud_run": {
			must(collector.NewCloudRunCollector(account)),
			disableCloudRunCollector,
		},
		"compute": {
			must(collector.NewComputeCollector(account)),
			disableComputeCollector,
		},
		"endpoints": {
			must(collector.NewEndpointsCollector(account)),
			disableEndpointsCollector,
		},
		"eventarc": {
			must(collector.NewEventarcCollector(account)),
			disableEventarcCollector,
		},
		"functions": {
			must(collector.NewFunctionsCollector(account)),
			disableFunctionsCollector,
		},
		"iam": {
			must(collector.NewIAMCollector(account)),
			disableIAMCollector,
		},
		"gke": {
			must(collector.NewGKECollector(account, *enableExtendedMetricsGKECollector)),
			disableGKECollector,
		},
		"logging": {
			must(collector.NewLoggingCollector(account)),
			disableLoggingCollector,
		},
		"monitoring": {
			must(collector.NewMonitoringCollector(account)),
			disableMonitoringCollector,
		},
		"scheduler": {
			must(collector.NewSchedulerCollector(account)),
			disableSchedulerCollector,
		},
		"storage": {
			must(collector.NewStorageCollector(account)),
			disableStorageCollector,
		},
	}

	for name, config := range collectorConfigs {
		if config.disable != nil && !*config.disable {
			log.Printf("Registering collector: %s", name)
			registry.MustRegister(config.collector)
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

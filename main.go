package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"google.golang.org/api/cloudresourcemanager/v1"

	"github.com/DazWilkin/gcp-exporter/collector"

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

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	fmt.Fprint(w, "<h2>Google Cloud Platform Resources Exporter</h2>")
	fmt.Fprint(w, "<ul>")
	fmt.Fprintf(w, "<li><a href=\"%s\">metrics</a></li>", *metricsPath)
	fmt.Fprintf(w, "<li><a href=\"/healthz\">healthz</a></li>")
	fmt.Fprint(w, "</ul>")
}
func main() {
	flag.Parse()

	if GitCommit == "" {
		log.Println("[main] GitCommit value unchanged: expected to be set during build")
	}
	if OSVersion == "" {
		log.Println("[main] OSVersion value unchanged: expected to be set during build")
	}

	registry := prometheus.NewRegistry()

	ctx := context.Background()
	cloudresourcemanagerService, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create the Projects.List request
	// Return at most (!) '--pagesize' projects
	// Filter the results to only include the project ID and number
	req := cloudresourcemanagerService.Projects.List().PageSize(*pagesize).Fields("projects.projectId", "projects.projectNumber")
	// Combine any user-specified filter with "lifecycleState:ACTIVE" to only process active projects
	if *filter != "" {
		*filter += " "
	}
	*filter = *filter + "lifecycleState:ACTIVE"
	req = req.Filter(*filter)
	log.Printf("[main] Projects filter: '%s'", *filter)
	req = req.Filter(*filter)

	resp, err := req.Context(ctx).Do()
	if err != nil {
		log.Fatal(err)
	}
	if resp.NextPageToken != "" {
		// There are more projects to return but we're limiting the results to this set
		log.Println("[main] Some projects are being excluded from the results. Refine 'filter' or increase 'max_projects'")
	}
	if len(resp.Projects) == 0 {
		log.Println("[main] There are 0 projects. Nothing to do")
		os.Exit(0)
	}

	log.Printf("[main] Exporting metrics for %d project(s)", len(resp.Projects))
	registry.MustRegister(collector.NewComputeCollector(resp.Projects))
	registry.MustRegister(collector.NewCloudRunCollector(resp.Projects))
	registry.MustRegister(collector.NewExporterCollector(OSVersion, GoVersion, GitCommit, StartTime))
	registry.MustRegister(collector.NewKubernetesCollector(resp.Projects))
	registry.MustRegister(collector.NewStorageCollector(resp.Projects))

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(handleRoot))
	mux.Handle("/healthz", http.HandlerFunc(handleHealthz))
	mux.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	log.Printf("[main] Server starting (%s)", *endpoint)
	log.Printf("[main] metrics served on: %s", *metricsPath)
	log.Fatal(http.ListenAndServe(*endpoint, mux))
}

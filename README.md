# Prometheus Exporter for [Google Cloud Platform (GCP)](https://cloud.google.com/) resources

I want to be able to monitor my resource consumption across multiple cloud platforms (GCP, Digital Ocean and Linode). I was inspired by [@metalmatze](https://github.com/metalmatze)'s [DigitalOcean Exporter](https://github.com/metalmatze/digitalocean_exporter) and, with this exporter, have the three that I need:

+ [Google Cloud Platform Exporter](https://github.com/DazWilkin/gcp-exporter)
+ [Digital Ocean Exporter](https://github.com/metalmatze/digitalocean_exporter)
+ [Linode Exporter](https://github.com/DazWilkin/linode-exporter)

## Installation


## Run


## Develop

## Metrics

| Name                                       | Type    | Description
| ----                                       | ------- | -----------
| `gcp_exporter_buildinfo`                   | Counter ||
| `gcp_exporter_startime`                    | Gauge   ||
| `gcp_compute_engine_instances`             | Gauge   ||
| `gcp_kubernetes_engine_cluster_up`         | Gauge   ||
| `gcp_kubernetes_engine_cluster_nodes`      | Gauge   ||

## Port

Registered `9402` with Prometheus Exporters' [default port allocations](https://github.com/prometheus/prometheus/wiki/Default-port-allocations#exporters-starting-at-9100)

## References

Using Google's (now legacy) API Client Libraries. The current Cloud Client Libraries do not provide coverage for all the relevant resources.

+ Google [Compute Engine API](https://cloud.google.com/compute/docs/reference/rest/)
+ Google [Resource Manager API](https://cloud.google.com/resource-manager/reference/rest/) && [GoDoc](https://godoc.org/google.golang.org/api/cloudresourcemanager/v1)
+ Google [Kubernetes Engine (Container) API](https://cloud.google.com/kubernetes-engine/docs/reference/rest/) && [GoDoc](https://godoc.org/google.golang.org/api/container/v1)
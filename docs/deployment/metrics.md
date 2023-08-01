---
title: "Metrics"
layout: default
sort: 7
---

# Metrics

Metrics are configured to be exposed using [prometheus operator](https://github.com/prometheus-operator/prometheus-operator)
API's by default. If you want to expose metrics using the prometheus operator
API's you need to install the prometheus operator in your cluster.
By default NFD Master and Worker expose metrics on port 8081.

The exposed metrics are

| Metric                                            | Type      | Description
| ------------------------------------------------- | --------- | ---------------------------------------
| `nfd_master_build_info`                           | Gauge     | Version from which nfd-master was built
| `nfd_worker_build_info`                           | Gauge     | Version from which nfd-worker was built
| `nfd_node_updates_total`                          | Counter   | Number of nodes updated
| `nfd_node_update_failures_total`                  | Counter   | Number of nodes update failures
| `nfd_nodefeaturerule_processing_duration_seconds` | Histogram | Time taken to process NodeFeatureRule objects
| `nfd_nodefeaturerule_processing_errors_total`     | Counter   | Number or errors encountered while processing NodeFeatureRule objects
| `nfd_feature_discovery_duration_seconds`          | Histogram | Time taken to discover features on a node

## Via Kustomize

To deploy NFD with metrics enabled using kustomize, you can use the
[Metrics Overlay](kustomize.md#metrics).

## Via Helm

By default metrics are enabled when deploying NFD via Helm. To enable Prometheus
to scrape metrics from NFD, you need to pass the following values to Helm:

```bash
--set prometheus.enable=true
```

For more info on Helm deployment, see [Helm](helm.md).

We recommend setting
`--set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false`
when deploying prometheus-operator via Helm to enable the prometheus-operator
to scrape metrics from any PodMonitor.

or setting labels on the PodMonitor via the helm parameter `prometheus.labels`
to control which Prometheus instances will scrape this PodMonitor.

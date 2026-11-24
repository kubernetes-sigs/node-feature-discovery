---
title: "Metrics"
layout: default
sort: 6
---

# Metrics

All NFD components expose Prometheus metrics on their `/metrics` HTTP endpoint
by default. Collecting them through the
[prometheus operator](https://github.com/prometheus-operator/prometheus-operator)
is opt-in: it creates a `PodMonitor` object, which requires the
prometheus-operator CRDs to be installed in the cluster.
By default nfd-master, nfd-worker, nfd-gc and nfd-topology-updater serve
Prometheus metrics at `/metrics` on port 8080. This is the same HTTP port that
serves `/healthz`, and it is set with each daemon's `-port` command-line flag.
In Helm, `master.port`, `worker.port` and `topologyUpdater.port` set that flag.

The exposed metrics are

| Metric                                                   | Type      | Description                                                                |
| -------------------------------------------------------- | --------- | -------------------------------------------------------------------------- |
| `nfd_master_build_info`                                  | Gauge     | Version from which nfd-master was built                                    |
| `nfd_worker_build_info`                                  | Gauge     | Version from which nfd-worker was built                                    |
| `nfd_gc_build_info`                                      | Gauge     | Version from which nfd-gc was built                                        |
| `nfd_topology_updater_build_info`                        | Gauge     | Version from which nfd-topology-updater was built                          |
| `nfd_master_node_update_requests_total`                  | Counter   | Number of node update requests processed by the master                     |
| `nfd_master_node_updates_total`                          | Counter   | Number of nodes updated                                                    |
| `nfd_master_node_feature_group_update_requests_total`    | Counter   | Number of cluster feature update requests processed by the master          |
| `nfd_master_node_update_failures_total`                  | Counter   | Number of nodes update failures                                            |
| `nfd_master_node_labels_rejected_total`                  | Counter   | Number of nodes labels rejected by nfd-master                              |
| `nfd_master_node_extendedresources_rejected_total`       | Counter   | Number of nodes extended resources rejected by nfd-master                  |
| `nfd_master_node_taints_rejected_total`                  | Counter   | Number of nodes taints rejected by nfd-master                              |
| `nfd_master_nodefeaturerule_processing_duration_seconds` | Histogram | Time taken to process NodeFeatureRule objects                              |
| `nfd_master_nodefeaturerule_processing_errors_total`     | Counter   | Number or errors encountered while processing NodeFeatureRule objects      |
| `nfd_worker_feature_discovery_duration_seconds`          | Histogram | Time taken to discover features on a node                                  |
| `nfd_topology_updater_scan_errors_total`                 | Counter   | Number of errors in scanning resource allocation of pods.                  |
| `nfd_gc_objects_deleted_total`                           | Counter   | Number of NodeFeature and NodeResourceTopology objects garbage collected.  |
| `nfd_gc_object_delete_failures_total`                    | Counter   | Number of errors in deleting NodeFeature and NodeResourceTopology objects. |

## Kustomize

To deploy NFD with metrics enabled using kustomize, you can use the
[prometheus overlay](kustomize.md#metrics).

## Helm

By default metrics are enabled when deploying NFD via Helm. To enable Prometheus
to scrape metrics from NFD, you need to pass the following values to Helm:

```bash
--set prometheus.enable=true
```

By default the PodMonitor scrapes all NFD components. To scrape only some of
them, list their `role` label values in `prometheus.components`. Valid values
are `master`, `worker`, `topology-updater` and `gc`. For example, to scrape only
nfd-master and nfd-gc:

```bash
--set prometheus.enable=true --set 'prometheus.components={master,gc}'
```

For more info on Helm deployment, see [Helm](helm.md).

It is recommended to specify
`--set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false`
when deploying prometheus-operator via Helm to enable the prometheus-operator
to scrape metrics from any PodMonitor.

or setting labels on the PodMonitor via the helm parameter `prometheus.labels`
to control which Prometheus instances will scrape this PodMonitor.

## Serving metrics over HTTPS

By default the metrics and health endpoints use plain HTTP. All four daemons
can serve them over HTTPS instead, using a certificate and private key from
files. Both endpoints share one port, so `/healthz` moves to HTTPS together
with `/metrics`.

HTTPS is enabled with two command-line flags:

- `-tls-cert-file` is the path to the PEM-encoded certificate.
- `-tls-private-key-file` is the path to the PEM-encoded private key.

Set both flags or neither. A daemon exits at startup if only one of them is
set, or if the files are missing, unreadable, not PEM encoded, or not a
matching pair. It does not fall back to plain HTTP.

The server accepts TLS 1.3 or later and speaks HTTP/1.1 only. FIPS-only Go
clients built with Go 1.21 to 1.23 support only TLS 1.2, so they cannot
connect.

Each failed TLS handshake adds a warning to the daemon's log. A TCP health
check that connects and closes without a handshake counts as a failed
handshake, so it adds one warning per check.

A daemon reads both files again on every new connection. After a certificate
is renewed, the next connection gets the new one, with no restart. If the files
stop being a valid pair, the daemon logs an error and keeps serving the last
certificate that loaded until the files are fixed. Probes keep passing, so the
pods do not restart. Scrapers that verify the certificate fail once it expires.

### Enabling HTTPS with Helm

To serve metrics over HTTPS with Helm, point `tls.secretName` at a Secret that
holds `tls.crt` and `tls.key`. The Secret must be in the release namespace and
must exist before the pods can start. The same Secret serves all components.

```bash
--set tls.enable=true --set tls.secretName=nfd-metrics-tls
```

Helm mounts the Secret read-only at `/etc/kubernetes/node-feature-discovery/certs`
in every component and sets both flags. The startup, liveness and readiness
probes switch to HTTPS, and the PodMonitor scrapes with the `https` scheme.
The kubelet does not verify the certificate of an HTTPS probe.

The Secret is typically written by [cert-manager](https://cert-manager.io/).
For example, this `Certificate` creates the `nfd-metrics-tls` Secret in the
`node-feature-discovery` namespace and renews it before it expires:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: nfd-metrics
  namespace: node-feature-discovery
spec:
  secretName: nfd-metrics-tls
  dnsNames:
    - nfd-metrics.node-feature-discovery.svc
  issuerRef:
    name: my-issuer
    kind: Issuer
```

### Scraping over TLS

The PodMonitor reaches each pod by its IP address. Pod IPs change, so the
certificate does not name the address Prometheus connects to. Use
`prometheus.tlsConfig` to tell Prometheus how to check it. Helm copies the
value verbatim to the PodMonitor's `podMetricsEndpoints[].tlsConfig`.

To verify the certificate, set `serverName` to a DNS name in the certificate
and give Prometheus the CA that signed it. The name does not need to resolve.

```yaml
prometheus:
  enable: true
  tlsConfig:
    serverName: nfd-metrics.node-feature-discovery.svc
    ca:
      secret:
        name: nfd-metrics-tls
        key: ca.crt
```

The `ca.crt` key is present when the issuer provides a CA certificate, as the
cert-manager CA issuer does.

To skip verification instead, use:

```yaml
prometheus:
  enable: true
  tlsConfig:
    insecureSkipVerify: true
```

## Grafana dashboard

NFD contains an example Grafana dashboard. You can import
[`examples/grafana-dashboard.json`](https://raw.githubusercontent.com/kubernetes-sigs/node-feature-discovery/{{site.release}}/examples/grafana-dashboard.json)
to your Grafana instance to visualize the NFD metrics.

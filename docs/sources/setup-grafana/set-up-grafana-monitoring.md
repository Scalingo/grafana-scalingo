---
aliases:
  - /docs/grafana/latest/admin/metrics/
  - /docs/grafana/latest/administration/jaeger-instrumentation/
  - /docs/grafana/latest/administration/view-server/internal-metrics/
  - /docs/grafana/latest/setup-grafana/set-up-grafana-monitoring/
description: Jaeger traces emitted and propagation by Grafana
keywords:
  - grafana
  - jaeger
  - tracing
title: Set up Grafana monitoring
weight: 800
---

# Set up Grafana monitoring

Grafana supports [Jaeger tracing](https://www.jaegertracing.io/).

Grafana can emit Jaeger traces for its HTTP API endpoints and propagate Jaeger trace information to data sources.
All HTTP endpoints are logged evenly (annotations, dashboard, tags, and so on).
When a trace ID is propagated, it is reported with operation 'HTTP /datasources/proxy/:id/\*'.

Refer to [Configuration]({{< relref "configure-grafana/#tracingjaeger" >}}) for information about enabling Jaeger tracing.

## View Grafana internal metrics

Grafana collects some metrics about itself internally. Grafana supports pushing metrics to Graphite or exposing them to be scraped by Prometheus.

For more information about configuration options related to Grafana metrics, refer to [metrics]({{< relref "configure-grafana/#metrics" >}}) and [metrics.graphite]({{< relref "configure-grafana/#metricsgraphite" >}}) in [Configuration]({{< relref "configure-grafana/" >}}).

### Available metrics

When enabled, Grafana exposes a number of metrics, including:

- Active Grafana instances
- Number of dashboards, users, and playlists
- HTTP status codes
- Requests by routing group
- Grafana active alerts
- Grafana performance

### Pull metrics from Grafana into Prometheus

These instructions assume you have already added Prometheus as a data source in Grafana.

1. Enable Prometheus to scrape metrics from Grafana. In your configuration file (`grafana.ini` or `custom.ini` depending on your operating system) remove the semicolon to enable the following configuration options:

   ```
   # Metrics available at HTTP URL /metrics and /metrics/plugins/:pluginId
   [metrics]
   # Disable / Enable internal metrics
   enabled           = true

   # Disable total stats (stat_totals_*) metrics to be generated
   disable_total_stats = false
   ```

1. (optional) If you want to require authorization to view the metrics endpoints, then uncomment and set the following options:

   ```
   basic_auth_username =
   basic_auth_password =
   ```

1. Restart Grafana. Grafana now exposes metrics at http://localhost:3000/metrics.
1. Add the job to your prometheus.yml file.
   Example:

   ```
   - job_name: 'grafana_metrics'

      scrape_interval: 15s
      scrape_timeout: 5s

      static_configs:
        - targets: ['localhost:3000']
   ```

1. Restart Prometheus. Your new job should appear on the Targets tab.
1. In Grafana, hover your mouse over the **Configuration** (gear) icon on the left sidebar and then click **Data Sources**.
1. Select the **Prometheus** data source.
1. On the Dashboards tab, **Import** the Grafana metrics dashboard. All scraped Grafana metrics are available in the dashboard.

### View Grafana metrics in Graphite

These instructions assume you have already added Graphite as a data source in Grafana.

1. Enable sending metrics to Graphite. In your configuration file (`grafana.ini` or `custom.ini` depending on your operating system) remove the semicolon to enable the following configuration options:

   ```
   # Metrics available at HTTP API Url /metrics
   [metrics]
   # Disable / Enable internal metrics
   enabled           = true

   # Disable total stats (stat_totals_*) metrics to be generated
   disable_total_stats = false
   ```

1. Enable [metrics.graphite] options:

   ```
   # Send internal metrics to Graphite
   [metrics.graphite]
   # Enable by setting the address setting (ex localhost:2003)
   address = <hostname or ip>:<port#>
   prefix = prod.grafana.%(instance_name)s.
   ```

1. Restart Grafana. Grafana now exposes metrics at http://localhost:3000/metrics and sends them to the Graphite location you specified.

### Pull metrics from Grafana backend plugin into Prometheus

Any installed [backend plugin]({{< relref "../developers/plugins/backend/" >}}) exposes a metrics endpoint through Grafana that you can configure Prometheus to scrape.

These instructions assume you have already added Prometheus as a data source in Grafana.

1. Enable Prometheus to scrape backend plugin metrics from Grafana. In your configuration file (`grafana.ini` or `custom.ini` depending on your operating system) remove the semicolon to enable the following configuration options:

   ```
   # Metrics available at HTTP URL /metrics and /metrics/plugins/:pluginId
   [metrics]
   # Disable / Enable internal metrics
   enabled           = true

   # Disable total stats (stat_totals_*) metrics to be generated
   disable_total_stats = false
   ```

1. (optional) If you want to require authorization to view the metrics endpoints, then uncomment and set the following options:

   ```
   basic_auth_username =
   basic_auth_password =
   ```

1. Restart Grafana. Grafana now exposes metrics at `http://localhost:3000/metrics/plugins/<plugin id>`, e.g. http://localhost:3000/metrics/plugins/grafana-github-datasource if you have the [Grafana GitHub datasource](https://grafana.com/grafana/plugins/grafana-github-datasource/) installed.
1. Add the job to your prometheus.yml file.
   Example:

   ```
   - job_name: 'grafana_github_datasource'

      scrape_interval: 15s
      scrape_timeout: 5s
      metrics_path: /metrics/plugins/grafana-test-datasource

      static_configs:
        - targets: ['localhost:3000']
   ```

1. Restart Prometheus. Your new job should appear on the Targets tab.
1. In Grafana, hover your mouse over the **Configuration** (gear) icon on the left sidebar and then click **Data Sources**.
1. Select the **Prometheus** data source.
1. Import a Golang application metrics dashboard - for example [Go Processes](https://grafana.com/grafana/dashboards/6671).

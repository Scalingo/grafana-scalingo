+++
title = "Data Sources"
type = "docs"
[menu.docs]
name = "Data Sources"
identifier = "datasources"
parent = "features"
weight = 5
+++

# Data Source Overview

Grafana supports many different storage backends for your time series data (Data Source). Each Data Source has a specific Query Editor that is customized for the features and capabilities that the particular Data Source exposes.


## Querying

The query language and capabilities of each Data Source are obviously very different. You can combine data from multiple Data Sources onto a single Dashboard, but each Panel is tied to a specific Data Source that belongs to a particular Organization.

## Supported Data Sources

The following datasources are officially supported:

* [Graphite]({{< relref "graphite.md" >}})
* [Elasticsearch]({{< relref "elasticsearch.md" >}})
* [CloudWatch]({{< relref "cloudwatch.md" >}})
* [InfluxDB]({{< relref "influxdb.md" >}})
* [OpenTSDB]({{< relref "opentsdb.md" >}})
* [Prometheus]({{< relref "prometheus.md" >}})

## Data source plugins

Since grafana 3.0 you can install data sources as plugins. Checkout [Grafana.net](https://grafana.net/plugins) for more data sources.


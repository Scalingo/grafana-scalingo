![Grafana](docs/logo-horizontal.png)

The open-source platform for monitoring and observability

Run your own Grafana instance with one click.

To login, use the environment variables value defined in:

* `GF_SECURITY_ADMIN_USER`
* `GF_SECURITY_ADMIN_PASSWORD`

Default Version: v9.3.1

[![Deploy](https://cdn.scalingo.com/deploy/button.svg)](https://dashboard.scalingo.com/create/app?source=https://github.com/Scalingo/grafana-scalingo)

## How to update when new version is released:

```
git clone git@github.com:Scalingo/grafana-scalingo
cd grafana-scalingo
git remote add scalingo git@ssh.osc-fr1.scalingo.com:<app name>.git
git push scalingo master
```

## How to update to the latest Grafana version

Get the latest version from upstream:

```
git remote add upstream https://github.com/grafana/grafana.git
git fetch upstream --tags
git checkout -b deps/grafana_vX.Y.Z
git merge vX.Y.Z
git commit --message "Get vX.Y.Z from upstream"
```

Open a pull request and get it merged. Then tag a new version:

```
git tag vX.Y.Z-scalingo1
git push origin --tags
```

[![License](https://img.shields.io/github/license/grafana/grafana)](LICENSE)
[![Drone](https://drone.grafana.net/api/badges/grafana/grafana/status.svg)](https://drone.grafana.net/grafana/grafana)
[![Go Report Card](https://goreportcard.com/badge/github.com/grafana/grafana)](https://goreportcard.com/report/github.com/grafana/grafana)

Grafana allows you to query, visualize, alert on and understand your metrics no matter where they are stored. Create, explore, and share dashboards with your team and foster a data-driven culture:

- **Visualizations:** Fast and flexible client side graphs with a multitude of options. Panel plugins offer many different ways to visualize metrics and logs.
- **Dynamic Dashboards:** Create dynamic & reusable dashboards with template variables that appear as dropdowns at the top of the dashboard.
- **Explore Metrics:** Explore your data through ad-hoc queries and dynamic drilldown. Split view and compare different time ranges, queries and data sources side by side.
- **Explore Logs:** Experience the magic of switching from metrics to logs with preserved label filters. Quickly search through all your logs or streaming them live.
- **Alerting:** Visually define alert rules for your most important metrics. Grafana will continuously evaluate and send notifications to systems like Slack, PagerDuty, VictorOps, OpsGenie.
- **Mixed Data Sources:** Mix different data sources in the same graph! You can specify a data source on a per-query basis. This works for even custom datasources.

## Get started

- [Get Grafana](https://grafana.com/get)
- [Installation guides](https://grafana.com/docs/grafana/latest/setup-grafana/installation/)

Unsure if Grafana is for you? Watch Grafana in action on [play.grafana.org](https://play.grafana.org/)!

## Documentation

The Grafana documentation is available at [grafana.com/docs](https://grafana.com/docs/).

## Contributing

If you're interested in contributing to the Grafana project:

- Start by reading the [Contributing guide](https://github.com/grafana/grafana/blob/HEAD/CONTRIBUTING.md).
- Learn how to set up your local environment, in our [Developer guide](https://github.com/grafana/grafana/blob/HEAD/contribute/developer-guide.md).
- Explore our [beginner-friendly issues](https://github.com/grafana/grafana/issues?q=is%3Aopen+is%3Aissue+label%3A%22beginner+friendly%22).
- Look through our [style guide and Storybook](https://developers.grafana.com/ui/latest/index.html).

## Get involved

- Follow [@grafana on Twitter](https://twitter.com/grafana/).
- Read and subscribe to the [Grafana blog](https://grafana.com/blog/).
- If you have a specific question, check out our [discussion forums](https://community.grafana.com/).
- For general discussions, join us on the [official Slack](https://slack.grafana.com) team.

## License

Grafana is distributed under [AGPL-3.0-only](LICENSE). For Apache-2.0 exceptions, see [LICENSING.md](https://github.com/grafana/grafana/blob/HEAD/LICENSING.md).

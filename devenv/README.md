# Set up your development environment

This folder contains useful scripts and configuration so you can:

* Configure data sources in Grafana for development.
* Configure dashboards for development and test scenarios.
* Create docker-compose file with databases and fake data.

## Install Docker

Grafana uses [Docker](https://docker.com) to make the task of setting up databases a little easier. If you do not have it already, make sure you [install Docker](https://docs.docker.com/docker-for-mac/install/) before proceeding to the next step.

## Developer dashboards and data sources

```bash
./setup.sh
```

After restarting the Grafana server, there should be a number of data sources named `gdev-<type>` provisioned as well as
a dashboard folder named `gdev dashboards`. This folder contains dashboard and panel features tests dashboards. 

Please update these dashboards or make new ones as new panels and dashboards features are developed or new bugs are
found. The dashboards are located in the `devenv/dev-dashboards` folder. 

## docker-compose with databases

This command creates a docker-compose file with specified databases configured and ready to run. Each database has
a prepared image with some fake data ready to use. For available databases, see `docker/blocks` directory. Notice that
for some databases there are multiple images, for example there is prometheus_mac specifically for Macs or different
version.

```bash
make devenv sources=influxdb,prometheus2,elastic5
```

Some of the blocks support dynamic change of the image version used in the Docker file. The signature looks like this: 

```bash
make devenv sources=postgres,openldap postgres_version=9.2
```

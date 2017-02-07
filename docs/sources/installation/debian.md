+++
title = "Installing on Debian / Ubuntu"
description = "Install guide for Grafana"
keywords = ["grafana", "installation", "documentation"]
type = "docs"
aliases = ["/installation/installation/debian"]
[menu.docs]
name = "Installing on Ubuntu / Debian"
identifier = "debian"
parent = "installation"
weight = 1
+++

# Installing on Debian / Ubuntu

Description | Download
------------ | -------------
Stable for Debian-based Linux | [4.0.2 (x86-64 deb)](https://grafanarel.s3.amazonaws.com/builds/grafana_4.0.2-1481203731_amd64.deb)
Latest beta for Debian-based Linux | [4.1.0-beta1 (x86-64 deb)](https://grafanarel.s3.amazonaws.com/builds/grafana_4.1.0-1482230757beta1_amd64.deb)

## Install Stable

```
$ wget https://grafanarel.s3.amazonaws.com/builds/grafana_4.0.2-1481203731_amd64.deb
$ sudo apt-get install -y adduser libfontconfig
$ sudo dpkg -i grafana_4.0.2-1481203731_amd64.deb
```

## Install Latest Beta

```
$ wget https://grafanarel.s3.amazonaws.com/builds/grafana_4.1.0-1482230757beta1_amd64.deb
$ sudo apt-get install -y adduser libfontconfig
$ sudo dpkg -i grafana_4.1.0-1482230757beta1_amd64.deb
```

## APT Repository

Add the following line to your `/etc/apt/sources.list` file.

    deb https://packagecloud.io/grafana/stable/debian/ jessie main

Use the above line even if you are on Ubuntu or another Debian version.
There is also a testing repository if you want beta or release
candidates.

    deb https://packagecloud.io/grafana/testing/debian/ jessie main

Then add the [Package Cloud](https://packagecloud.io/grafana) key. This
allows you to install signed packages.

    $ curl https://packagecloud.io/gpg.key | sudo apt-key add -

Update your Apt repositories and install Grafana

    $ sudo apt-get update
    $ sudo apt-get install grafana

On some older versions of Ubuntu and Debian you may need to install the
`apt-transport-https` package which is needed to fetch packages over
HTTPS.

    $ sudo apt-get install -y apt-transport-https

## Package details

- Installs binary to `/usr/sbin/grafana-server`
- Installs Init.d script to `/etc/init.d/grafana-server`
- Creates default file (environment vars) to `/etc/default/grafana-server`
- Installs configuration file to `/etc/grafana/grafana.ini`
- Installs systemd service (if systemd is available) name `grafana-server.service`
- The default configuration sets the log file at `/var/log/grafana/grafana.log`
- The default configuration specifies an sqlite3 db at `/var/lib/grafana/grafana.db`

## Start the server (init.d service)

Start Grafana by running:

    $ sudo service grafana-server start

This will start the `grafana-server` process as the `grafana` user,
which was created during the package installation. The default HTTP port
is `3000` and default user and group is `admin`.

To configure the Grafana server to start at boot time:

    $ sudo update-rc.d grafana-server defaults

## Start the server (via systemd)

To start the service using systemd:

    $ systemctl daemon-reload
    $ systemctl start grafana-server
    $ systemctl status grafana-server

Enable the systemd service so that Grafana starts at boot.

    sudo systemctl enable grafana-server.service

## Environment file

The systemd service file and init.d script both use the file located at
`/etc/default/grafana-server` for environment variables used when
starting the back-end. Here you can override log directory, data
directory and other variables.

### Logging

By default Grafana will log to `/var/log/grafana`

### Database

The default configuration specifies a sqlite3 database located at
`/var/lib/grafana/grafana.db`. Please backup this database before
upgrades. You can also use MySQL or Postgres as the Grafana database, as detailed on [the configuration page]({{< relref "configuration.md#database" >}}).

## Configuration

The configuration file is located at `/etc/grafana/grafana.ini`.  Go the
[Configuration]({{< relref "configuration.md" >}}) page for details on all
those options.

### Adding data sources

- [Graphite]({{< relref "datasources/graphite.md" >}})
- [InfluxDB]({{< relref "datasources/influxdb.md" >}})
- [OpenTSDB]({{< relref "datasources/opentsdb.md" >}})
- [Prometheus]({{< relref "datasources/prometheus.md" >}})

## Installing from binary tar file

Download [the latest `.tar.gz` file](http://grafana.org/download) and
extract it.  This will extract into a folder named after the version you
downloaded. This folder contains all files required to run Grafana.  There are
no init scripts or install scripts in this package.

To configure Grafana add a configuration file named `custom.ini` to the
`conf` folder and override any of the settings defined in
`conf/defaults.ini`.

Start Grafana by executing `./bin/grafana-server web`. The `grafana-server`
binary needs the working directory to be the root install directory (where the
binary and the `public` folder is located).

---
aliases:
  - /docs/grafana/latest/installation/debian/
  - /docs/grafana/latest/installation/installation/debian/
  - /docs/grafana/latest/setup-grafana/installation/debian/
description: Install guide for Grafana on Debian or Ubuntu
title: Install on Debian or Ubuntu
weight: 100
---

# Install on Debian or Ubuntu

This page explains how to install Grafana dependencies, download and install Grafana, get the service up and running on your Debian or Ubuntu system, and also describes the installation package details.

## Repository migration (November 8th 2022)

From that date, Grafana packages will be served from a new repository (<packages.grafana.com/deb/{product}> -> <apt.grafana.com>). The new repository serves, from a single APT configuration, all Grafana OSS products, as well as Grafana Enterprise.

The old URLs will still work, serving the content from the new repository, but you may encounter warnings about some repository attributes changing (e.g. `Origin` and `Label`).

## Note on upgrading

<<<<<<<< HEAD:docs/sources/setup-grafana/installation/debian.md
While the process for upgrading Grafana is very similar to installing Grafana, there are some key backup steps you should perform. Read [Upgrading Grafana]({{< relref "../upgrade-grafana/" >}}) for tips and guidance on updating an existing installation.

> **Note:** You can use [Grafana Cloud](https://grafana.com/products/cloud/features/#cloud-logs) to avoid the overhead of installing, maintaining, and scaling your observability stack. The free forever plan includes Grafana, 10K Prometheus series, 50 GB logs, and more.[Create a free account to get started](https://grafana.com/auth/sign-up/create-user?pg=docs-grafana-install&plcmt=in-text).
========
While the process for upgrading Grafana is very similar to installing Grafana, there are some key backup steps you should perform. Read [Upgrading Grafana]({{< relref "../../upgrade-grafana/" >}}) for tips and guidance on updating an existing installation.
>>>>>>>> v9.3.1:docs/sources/setup-grafana/installation/debian/index.md

## 1. Download and install

You can install Grafana using our official APT repository, by downloading a `.deb` package, or by downloading a binary `.tar.gz` file.

### Install from APT repository

If you install from the APT repository, then Grafana is automatically updated every time you run `apt-get update`.

| Grafana Version           | Package            | Repository                            |
| ------------------------- | ------------------ | ------------------------------------- |
| Grafana Enterprise        | grafana-enterprise | `https://apt.grafana.com stable main` |
| Grafana Enterprise (Beta) | grafana-enterprise | `https://apt.grafana.com beta main`   |
| Grafana OSS               | grafana            | `https://apt.grafana.com stable main` |
| Grafana OSS (Beta)        | grafana            | `https://apt.grafana.com beta main`   |

> **Note:** Grafana Enterprise is the recommended and default edition. It is available for free and includes all the features of the OSS edition. You can also upgrade to the [full Enterprise feature set](https://grafana.com/products/enterprise/?utm_source=grafana-install-page), which has support for [Enterprise plugins](https://grafana.com/grafana/plugins/?enterprise=1&utcm_source=grafana-install-page).

#### To install the latest release:

```bash
sudo apt-get install -y apt-transport-https
sudo apt-get install -y software-properties-common wget
sudo wget -q -O /usr/share/keyrings/grafana.key https://apt.grafana.com/gpg.key
```

Add this repository for stable releases:

```bash
echo "deb [signed-by=/usr/share/keyrings/grafana.key] https://apt.grafana.com stable main" | sudo tee -a /etc/apt/sources.list.d/grafana.list
```

Add this repository if you want beta releases:

```bash
echo "deb [signed-by=/usr/share/keyrings/grafana.key] https://apt.grafana.com beta main" | sudo tee -a /etc/apt/sources.list.d/grafana.list
```

After you add the repository:

```bash
sudo apt-get update

# Install the latest OSS release:
sudo apt-get install grafana

# Install the latest Enterprise release:
sudo apt-get install grafana-enterprise
```

### Install .deb package

If you install the `.deb` package, then you will need to manually update Grafana for each new version.

1. On the [Grafana download page](https://grafana.com/grafana/download), select the Grafana version you want to install.
   - The most recent Grafana version is selected by default.
   - The **Version** field displays only finished releases. If you want to install a beta version, click **Nightly Builds** and then select a version.
1. Select an **Edition**.
   - **Enterprise** - Recommended download. Functionally identical to the open source version, but includes features you can unlock with a license if you so choose.
   - **Open Source** - Functionally identical to the Enterprise version, but you will need to download the Enterprise version if you want Enterprise features.
1. Depending on which system you are running, click **Linux** or **ARM**.
1. Copy and paste the code from the installation page into your command line and run. It follows the pattern shown below.

```bash
sudo apt-get install -y adduser
wget <.deb package url>
sudo dpkg -i grafana<edition>_<version>_amd64.deb
```

## Install from binary .tar.gz file

Download the latest [`.tar.gz` file](https://grafana.com/grafana/download?platform=linux) and extract it. The files extract into a folder named after the Grafana version downloaded. This folder contains all files required to run Grafana. There are no init scripts or install scripts in this package.

```bash
wget <tar.gz package url>
sudo tar -zxvf <tar.gz package>
```

## 2. Start the server

This starts the `grafana-server` process as the `grafana` user, which was created during the package installation.

If you installed with the APT repository or `.deb` package, then you can start the server using `systemd` or `init.d`. If you installed a binary `.tar.gz` file, then you need to execute the binary.

### Start the server with systemd

To start the service and verify that the service has started:

```bash
sudo systemctl daemon-reload
sudo systemctl start grafana-server
sudo systemctl status grafana-server
```

Configure the Grafana server to start at boot:

```bash
sudo systemctl enable grafana-server.service
```

#### Serving Grafana on a port < 1024

{{< docs/shared "systemd/bind-net-capabilities.md" >}}

### Start the server with init.d

To start the service and verify that the service has started:

```bash
sudo service grafana-server start
sudo service grafana-server status
```

Configure the Grafana server to start at boot:

```bash
sudo update-rc.d grafana-server defaults
```

### Execute the binary

The `grafana-server` binary .tar.gz needs the working directory to be the root install directory where the binary and the `public` folder are located.

Start Grafana by running:

```bash
./bin/grafana-server web
```

## Package details

- Installs binary to `/usr/sbin/grafana-server`
- Installs Init.d script to `/etc/init.d/grafana-server`
- Creates default file (environment vars) to `/etc/default/grafana-server`
- Installs configuration file to `/etc/grafana/grafana.ini`
- Installs systemd service (if systemd is available) name `grafana-server.service`
- The default configuration sets the log file at `/var/log/grafana/grafana.log`
- The default configuration specifies a SQLite3 db at `/var/lib/grafana/grafana.db`
- Installs HTML/JS/CSS and other Grafana files at `/usr/share/grafana`

## Next steps

<<<<<<<< HEAD:docs/sources/setup-grafana/installation/debian.md
Refer to the [Getting Started]({{< relref "../../getting-started/build-first-dashboard/" >}}) guide for information about logging in, setting up data sources, and so on.

## Configure Grafana

Refer to the [Configuration]({{< relref "../configure-grafana/" >}}) page for details on options for customizing your environment, logging, database, and so on.
========
Refer to the [Getting Started]({{< relref "../../../getting-started/build-first-dashboard/" >}}) guide for information about logging in, setting up data sources, and so on.

## Configure Grafana

Refer to the [Configuration]({{< relref "../../configure-grafana/" >}}) page for details on options for customizing your environment, logging, database, and so on.
>>>>>>>> v9.3.1:docs/sources/setup-grafana/installation/debian/index.md

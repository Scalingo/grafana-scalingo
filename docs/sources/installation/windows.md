+++
title = "Installing on Windows"
description = "Installing Grafana on Windows"
keywords = ["grafana", "configuration", "documentation", "windows"]
type = "docs"
[menu.docs]
parent = "installation"
weight = 3
+++

# Installing on Windows

Description | Download
------------ | -------------
Latest stable package for Windows | [grafana-5.1.0.windows-x64.zip](https://s3-us-west-2.amazonaws.com/grafana-releases/release/grafana-5.1.0.windows-x64.zip)

<!--
Latest beta package for Windows | [grafana.5.1.0-beta1.windows-x64.zip](https://s3-us-west-2.amazonaws.com/grafana-releases/release/grafana-5.0.0-beta5.windows-x64.zip)
-->

Read [Upgrading Grafana]({{< relref "installation/upgrading.md" >}}) for tips and guidance on updating an existing
installation.

## Configure

The zip file contains a folder with the current Grafana version. Extract
this folder to anywhere you want Grafana to run from.  Go into the
`conf` directory and copy `sample.ini` to `custom.ini`. You should edit
`custom.ini`, never `defaults.ini`.

The default Grafana port is `3000`, this port requires extra permissions
on windows. Edit `custom.ini` and uncomment the `http_port`
configuration option (`;` is the comment character in ini files) and change it to something like `8080` or similar.
That port should not require extra Windows privileges.

Start Grafana by executing `grafana-server.exe`, located in the `bin` directory, preferably from the
command line. If you want to run Grafana as windows service, download
[NSSM](https://nssm.cc/). It is very easy to add Grafana as a Windows
service using that tool.

Read more about the [configuration options]({{< relref "configuration.md" >}}).

## Building on Windows

The Grafana backend includes Sqlite3 which requires GCC to compile. So
in order to compile Grafana on Windows you need to install GCC. We
recommend [TDM-GCC](http://tdm-gcc.tdragon.net/download).

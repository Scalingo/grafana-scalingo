#!/bin/bash

if [ -e $GRAFANA_PLUGINS ] ; then
  echo "No plugins to install"
  exit 0
fi

OLD_IFS=$IFS
IFS=','

pluginDir="$(pwd)/data/plugins"
mkdir -p $pluginDir

for plugin in $GRAFANA_PLUGINS ; do
  echo "Installing $plugin"
  ./bin/grafana-cli --pluginsDir=$pluginDir plugins install $plugin
done

IFS=$OLD_IFS

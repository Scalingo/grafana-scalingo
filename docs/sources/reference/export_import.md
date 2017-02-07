+++
title = "Export & Import"
keywords = ["grafana", "dashboard", "documentation", "export", "import"]
type = "docs"
[menu.docs]
parent = "dashboard_features"
weight = 8
+++

# Export and Import

Grafana Dashboards can easily be exported and imported, either from the UI or from the HTTP API.

## Exporting a dashboard

Dashboards are exported in Grafana JSON format, and contain everything you need (layout, variables, styles, data sources, queries, etc)to import the dashboard at a later time.

The export feature is accessed from the share menu.

<img src="/img/docs/v31/export_menu.png">

### Making a dashboard portable

If you want to export a dashboard for others to use then it could be a good idea to
add template variables for things like a metric prefix (use constant variable) and server name.

A template variable of the type `Constant` will automatically be hidden in
the dashboard, and will also be added as an required input when the dashboard is imported.

## Importing a dashboard

To import a dashboard open dashboard search and then hit the import button.

<img src="/img/docs/v31/import_step1.png">

From here you can upload a dashboard json file, paste a [Grafana.net](https://grafana.net) dashboard
url or paste dashboard json text directly into the text area.

<img src="/img/docs/v31/import_step2.png">

In step 2 of the import process Grafana will let you change the name of the dashboard, pick what
data source you want the dashboard to use and specify any metric prefixes (if the dashboard use any).

## Discover dashboards on Grafana.net

Find dashboards for common server applications at [Grafana.net/dashboards](https://grafana.net/dashboards).

<img src="/img/docs/v31/gnet_dashboards_list.png">

## Import & Sharing with Grafana 2.x or 3.0

Dashboards on Grafana.net use a new feature in Grafana 3.1 that allows the import process
to update each panel so that they are using a data source of your choosing. If you are running a
Grafana version older than 3.1 then you might need to do some manual steps either
before or after import in order for the dashboard to work properly.

Dashboards exported from Grafana 3.1+ have a new json section `__inputs`
that define what data sources and metric prefixes the dashboard uses.

Example:
```json
{
  "__inputs": [
    {
      "name": "DS_GRAPHITE",
      "label": "graphite",
      "description": "",
      "type": "datasource",
      "pluginId": "graphite",
      "pluginName": "Graphite"
    },
    {
      "name": "VAR_PREFIX",
      "type": "constant",
      "label": "prefix",
      "value": "collectd",
      "description": ""
    }
  ],
}

```

These are then referenced in the dashboard panels like this:

```json
{
  "rows": [
      {
        "panels": [
          {
            "type": "graph",
            "datasource": "${DS_GRAPHITE}",
          }
        ]
      }
  ]
}
```

These inputs and their usage in data source properties are automatically added during export in Grafana 3.1.
If you run an older version of Grafana and want to share a dashboard on Grafana.net you need to manually
add the inputs and templatize the datasource properties like above.

If you want to import a dashboard from Grafana.net into an older version of Grafana then you can either import
it as usual and then update the data source option in the metrics tab so that the panel is using the correct
data source. Another alternative is to open the json file in a a text editor and update the data source properties
to value that matches a name of your data source.


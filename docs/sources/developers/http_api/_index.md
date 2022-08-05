---
aliases:
  - /docs/grafana/latest/developers/http_api/
  - /docs/grafana/latest/http_api/
  - /docs/grafana/latest/overview/
description: Grafana HTTP API
keywords:
  - grafana
  - http
  - documentation
  - api
  - overview
title: HTTP API
weight: 100
---

# HTTP API reference

The Grafana backend exposes an HTTP API, which is the same API that is used by the frontend to do everything from saving
dashboards, creating users, and updating data sources.

## HTTP APIs

- [Admin API]({{< relref "admin/" >}})
- [Alerting Provisioning API]({{< relref "alerting_provisioning/" >}})
- [Annotations API]({{< relref "annotations/" >}})
- [Authentication API]({{< relref "auth/" >}})
- [Dashboard API]({{< relref "dashboard/" >}})
- [Dashboard Permissions API]({{< relref "dashboard_permissions/" >}})
- [Dashboard Versions API]({{< relref "dashboard_versions/" >}})
- [Data source API]({{< relref "data_source/" >}})
- [Folder API]({{< relref "folder/" >}})
- [Folder Permissions API]({{< relref "folder_permissions/" >}})
- [Folder/Dashboard Search API]({{< relref "folder_dashboard_search/" >}})
- [Library Element API]({{< relref "library_element/" >}})
- [Organization API]({{< relref "org/" >}})
- [Other API]({{< relref "other/" >}})
- [Playlists API]({{< relref "playlist/" >}})
- [Preferences API]({{< relref "preferences/" >}})
- [Short URL API]({{< relref "short_url/" >}})
- [Snapshot API]({{< relref "snapshot/" >}})
- [Team API]({{< relref "team/" >}})
- [User API]({{< relref "user/" >}})

## Deprecated HTTP APIs

- [Alerting Notification Channels API]({{< relref "alerting_notification_channels/" >}})
- [Alerting API]({{< relref "alerting/" >}})

## Grafana Enterprise HTTP APIs

Grafana Enterprise includes all of the Grafana OSS APIs as well as those that follow:

- [Role-based access control API]({{< relref "access_control/" >}})
- [Data source permissions API]({{< relref "datasource_permissions/" >}})
- [External group sync API]({{< relref "external_group_sync/" >}})
- [License API]({{< relref "licensing/" >}})
- [Reporting API]({{< relref "reporting/" >}})

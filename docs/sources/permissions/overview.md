+++
title = "Overview"
description = "Overview for permissions"
keywords = ["grafana", "configuration", "documentation", "admin", "users", "datasources", "permissions"]
type = "docs"
aliases = ["/docs/grafana/latest/reference/admin", "/docs/grafana/latest/administration/permissions/"]
[menu.docs]
name = "Overview"
identifier = "overview-permissions"
parent = "permissions"
weight = 1
+++

# Permissions Overview

Grafana users have permissions that are determined by their:

- **Organization Role** (Admin, Editor, Viewer)
- Via **Team** memberships where the **Team** has been assigned specific permissions.
- Via permissions assigned directly to user (on folders, dashboards, data sources)
- The Grafana Admin (i.e. Super Admin) user flag.

## Grafana Admin

This admin flag makes a user a `Super Admin`. This means they can access the `Server Admin` views where all users and organizations can be administrated.

## Organization Roles

Users can be belong to one or more organizations. A user's organization membership is tied to a role that defines what the user is allowed to do
in that organization. Learn more about [Organization Roles]({{< relref "organization_roles.md" >}}).


## Dashboard and Folder Permissions

Dashboard and folder permissions allows you to remove the default role based permissions for Editors and Viewers and assign permissions to specific **Users** and **Teams**. Learn more about [Dashboard and Folder Permissions]({{< relref "dashboard_folder_permissions.md" >}}).

## Data source permissions

Per default, a data source in an organization can be queried by any user in that organization. For example a user with `Viewer` role can still
issue any possible query to a data source, not just those queries that exist on dashboards he/she has access to.

Data source permissions allows you to change the default permissions for data sources and restrict query permissions to specific **Users** and **Teams**. Read more about [data source permissions]({{< relref "datasource_permissions.md" >}}).

---
aliases:
  - /docs/grafana/latest/enterprise/
  - /docs/grafana/latest/introduction/grafana-enterprise/
description: Grafana Enterprise overview
title: Grafana Enterprise
weight: 200
---

# Grafana Enterprise

Grafana Enterprise is a commercial edition of Grafana that includes additional features not found in the open source version.

Building on everything you already know and love about Grafana open source, Grafana Enterprise includes [exclusive datasource plugins]({{< relref "#enterprise-plugins">}}) and [additional features]({{< relref "#enterprise-features">}}). You also get 24x7x365 support and training from the core Grafana team.

To learn more about Grafana Enterprise, refer to [our product page](https://grafana.com/enterprise).

## Enterprise features in Grafana Cloud

Many Grafana Enterprise features are also available in [Grafana Cloud]({{< ref "/docs/grafana-cloud" >}}) Pro and Advanced accounts. For details, refer to [the Grafana Cloud features table](https://grafana.com/pricing/#featuresTable) and [Enterprise features available to Grafana Cloud Pro and Advanced accounts]({{< ref "/docs/grafana-cloud/reference/enterprise-features" >}}).

## Authentication

Grafana Enterprise includes integrations with more ways to authenticate your users and enhanced authentication capabilities.

### Team sync

[Team sync]({{< relref "../setup-grafana/configure-security/configure-team-sync/" >}}) allows you to set up synchronization between teams in Grafana and teams in your auth provider so that your users automatically end up in the right team.

Supported auth providers:

- [Auth Proxy]({{< relref "../setup-grafana/configure-security/configure-authentication/auth-proxy/#team-sync-enterprise-only" >}})
- [Azure AD OAuth]({{< relref "../setup-grafana/configure-security/configure-authentication/azuread/#team-sync-enterprise-only" >}})
- [GitHub OAuth]({{< relref "../setup-grafana/configure-security/configure-authentication/github/#team-sync-enterprise-only" >}})
- [GitLab OAuth]({{< relref "../setup-grafana/configure-security/configure-authentication/gitlab/#team-sync-enterprise-only" >}})
- [LDAP]({{< relref "../setup-grafana/configure-security/configure-authentication/enhanced_ldap/#ldap-group-synchronization-for-teams" >}})
- [Okta]({{< relref "../setup-grafana/configure-security/configure-authentication/okta/#team-sync-enterprise-only" >}})
- [SAML]({{< relref "../setup-grafana/configure-security/configure-authentication/saml/#configure-team-sync" >}})

### Enhanced LDAP integration

With [enhanced LDAP integration]({{< relref "../setup-grafana/configure-security/configure-authentication/enhanced_ldap/" >}}), you can set up active LDAP synchronization.

### SAML authentication

[SAML authentication]({{< relref "../setup-grafana/configure-security/configure-authentication/saml/" >}}) enables users to authenticate with single sign-on services that use Security Assertion Markup Language (SAML).

## Enterprise features

Grafana Enterprise adds the following features:

- [Role-based access control]({{< relref "../enterprise/access-control/" >}}) to control access with role-based permissions.
- [Data source permissions]({{< relref "../administration/data-source-management/datasource-permissions.md" >}}) to restrict query access to specific teams and users.
- [Data source query caching]({{< relref "../enterprise/query-caching.md" >}}) to temporarily store query results in Grafana to reduce data source load and rate limiting.
- [Reporting]({{< relref "../enterprise/reporting.md" >}}) to generate a PDF report from any dashboard and set up a schedule to have it emailed to whoever you choose.
- [Export dashboard as PDF]({{< relref "../enterprise/export-pdf.md" >}})
- [Custom branding]({{< relref "../setup-grafana/configure-grafana/configure-custom-branding/" >}}) to customize Grafana from the brand and logo to the footer links.
- [Usage insights]({{< relref "../enterprise/usage-insights/" >}}) to understand how your Grafana instance is used.
- [Vault integration]({{< relref "../setup-grafana/configure-security/configure-database-encryption/encrypt-secrets-using-hashicorp-key-vault/" >}}) to manage your configuration or provisioning secrets with Vault.
- [Auditing]({{< relref "../setup-grafana/configure-security/audit-grafana/" >}}) tracks important changes to your Grafana instance to help you manage and mitigate suspicious activity and meet compliance requirements.
- [Request security]({{< relref "../setup-grafana/configure-security/configure-request-security/" >}}) makes it possible to restrict outgoing requests from the Grafana server.
- [Settings updates at runtime]({{< relref "../enterprise/settings-updates.md" >}}) allows you to update Grafana settings at runtime without requiring a restart.

## Enterprise data sources

With a Grafana Enterprise license, you also get access to premium data sources, including:

- [AppDynamics](https://grafana.com/grafana/plugins/dlopes7-appdynamics-datasource)
- [Azure Devops](https://grafana.com/grafana/plugins/grafana-azuredevops-datasource)
- [DataDog](https://grafana.com/grafana/plugins/grafana-datadog-datasource)
- [Dynatrace](https://grafana.com/grafana/plugins/grafana-dynatrace-datasource)
- [Gitlab](https://grafana.com/grafana/plugins/grafana-gitlab-datasource)
- [Honeycomb](https://grafana.com/grafana/plugins/grafana-honeycomb-datasource)
- [Jira](https://grafana.com/grafana/plugins/grafana-jira-datasource)
- [MongoDB](https://grafana.com/grafana/plugins/grafana-mongodb-datasource)
- [New Relic](https://grafana.com/grafana/plugins/grafana-newrelic-datasource)
- [Oracle Database](https://grafana.com/grafana/plugins/grafana-oracle-datasource)
- [Salesforce](https://grafana.com/grafana/plugins/grafana-salesforce-datasource)
- [SAP HANA®](https://grafana.com/grafana/plugins/grafana-saphana-datasource)
- [ServiceNow](https://grafana.com/grafana/plugins/grafana-servicenow-datasource)
- [Snowflake](https://grafana.com/grafana/plugins/grafana-snowflake-datasource)
- [Splunk](https://grafana.com/grafana/plugins/grafana-splunk-datasource)
- [Splunk Infrastructure monitoring (SignalFx)](https://grafana.com/grafana/plugins/grafana-splunk-monitoring-datasource)
- [Wavefront](https://grafana.com/grafana/plugins/grafana-wavefront-datasource)

## Try Grafana Enterprise

To purchase or obtain a trial license, contact the Grafana Labs [Sales Team](https://grafana.com/contact?about=support&topic=Grafana%20Enterprise).

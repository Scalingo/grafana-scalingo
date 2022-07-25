---
aliases:
  - /docs/grafana/latest/administration/api-keys/about-api-keys/
  - /docs/grafana/latest/administration/api-keys/
  - /docs/grafana/latest/administration/api-keys/create-api-key/
description: This section contains information about API keys in Grafana
keywords:
  - API keys
  - Service accounts
menuTitle: API keys
title: API keys
weight: 700
---

# API keys

API keys can be used to interact with Grafana HTTP APIs.

We recommend using service accounts instead of API keys if you are on Grafana 8.5+, for more information refer to [About service accounts]({{< relref "../service-accounts/about-service-accounts/#" >}}).

{{< section >}}

## About API keys

An API key is a randomly generated string that external systems use to interact with Grafana HTTP APIs.

When you create an API key, you specify a **Role** that determines the permissions associated with the API key. Role permissions control that actions the API key can perform on Grafana resources. For more information about creating API keys, refer to [Create an API key]({{< relref "create-api-key/#" >}}).

## Create an API key

Create an API key when you want to manage your computed workload with a user.

For more information about API keys, refer to [About API keys in Grafana]({{< relref "about-api-keys/" >}}).

This topic shows you how to create an API key using the Grafana UI. You can also create an API key using the Grafana HTTP API. For more information about creating API keys via the API, refer to [Create API key via API]({{< relref "../../developers/http_api/create-api-tokens-for-org/#how-to-create-a-new-organization-and-an-api-token" >}}).

### Before you begin:

- Ensure you have permission to create and edit API keys. For more information about permissions, refer to [About users and permissions]({{< relref "../roles-and-permissions/#" >}}).

**To create an API key:**

1. Sign in to Grafana, hover your cursor over **Configuration** (the gear icon), and click **API Keys**.
1. Click **New API key**.
1. Enter a unique name for the key.
1. In the **Role** field, select one of the following access levels you want to assign to the key.
   - **Admin**: Enables a user to use APIs at the broadest, most powerful administrative level.
   - **Editor** or **Viewer** to limit the key's users to those levels of power.
1. In the **Time to live** field, specify how long you want the key to be valid.
   - The maximum length of time is 30 days (one month). You enter a number and a letter. Valid letters include `s` for seconds,`m` for minutes, `h` for hours, `d `for days, `w` for weeks, and `M `for month. For example, `12h` is 12 hours and `1M` is 1 month (30 days).
   - If you are unsure about how long an API key should be valid, we recommend that you choose a short duration, such as a few hours. This approach limits the risk of having API keys that are valid for a long time.
1. Click **Add**.

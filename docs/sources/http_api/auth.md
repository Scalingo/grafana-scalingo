+++
title = "Authentication HTTP API "
description = "Grafana Authentication HTTP API"
keywords = ["grafana", "http", "documentation", "api", "authentication"]
aliases = ["/http_api/authentication/"]
type = "docs"
[menu.docs]
name = "Authentication"
parent = "http_api"
+++

# Authentication API

## Tokens

Currently you can authenticate via an `API Token` or via a `Session cookie` (acquired using regular login or oauth).

## Basic Auth

If basic auth is enabled (it is enabled by default) you can authenticate your HTTP request via
standard basic auth. Basic auth will also authenticate LDAP users.

curl example:
```
?curl http://admin:admin@localhost:3000/api/org
{"id":1,"name":"Main Org."}
```

## Create API Token

Open the sidemenu and click the organization dropdown and select the `API Keys` option.

![](/img/docs/v2/orgdropdown_api_keys.png)

You use the token in all requests in the `Authorization` header, like this:

**Example**:

    GET http://your.grafana.com/api/dashboards/db/mydash HTTP/1.1
    Accept: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

The `Authorization` header value should be `Bearer <your api key>`.

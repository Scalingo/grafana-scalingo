+++
title = "User HTTP API "
description = "Grafana User HTTP API"
keywords = ["grafana", "http", "documentation", "api", "user"]
aliases = ["/http_api/user/"]
type = "docs"
[menu.docs]
name = "Users"
parent = "http_api"
+++

# User HTTP resources / actions

## Search Users

`GET /api/users`

**Example Request**:

    GET /api/users HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    [
      {
        "id": 1,
        "name": "Admin",
        "login": "admin",
        "email": "admin@mygraf.com",
        "isAdmin": true
      },
      {
        "id": 2,
        "name": "User",
        "login": "user",
        "email": "user@mygraf.com",
        "isAdmin": false
      }
    ]

## Get single user by Id

`GET /api/users/:id`

**Example Request**:

    GET /api/users/1 HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    {
      "email": "user@mygraf.com"
      "name": "admin",
      "login": "admin",
      "theme": "light",
      "orgId": 1,
      "isGrafanaAdmin": true
    }

## User Update

`PUT /api/users/:id`

**Example Request**:

    PUT /api/users/2 HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

    {
      "email":"user@mygraf.com",
      "name":"User2",
      "login":"user",
      "theme":"light"
    }

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    {"message":"User updated"}


## Get Organisations for user

`GET /api/users/:id/orgs`

**Example Request**:

    GET /api/users/1/orgs HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    [
      {
        "orgId":1,
        "name":"Main Org.",
        "role":"Admin"
      }
    ]

## User

## Actual User

`GET /api/user`

**Example Request**:

    GET /api/user HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    {
      "email":"admin@mygraf.com",
      "name":"Admin",
      "login":"admin",
      "theme":"light",
      "orgId":1,
      "isGrafanaAdmin":true
    }

## Change Password

`PUT /api/user/password`

Changes the password for the user

**Example Request**:

    PUT /api/user/password HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

    {
      "oldPassword": "old_password",
      "newPassword": "new_password",
      "confirmNew": "confirm_new_password"
    }

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    {"message":"User password changed"}

## Switch user context

`POST /api/user/using/:organisationId`

Switch user context to the given organisation.

**Example Request**:

    POST /api/user/using/2 HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    {"message":"Active organization changed"}

## Organisations of the actual User

`GET /api/user/orgs`

Return a list of all organisations of the current user.

**Example Request**:

    GET /api/user/orgs HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    [
      {
        "orgId":1,
        "name":"Main Org.",
        "role":"Admin"
      }
    ]

## Star a dashboard

`POST /api/user/stars/dashboard/:dashboardId`

Stars the given Dashboard for the actual user.

**Example Request**:

    POST /api/user/stars/dashboard/1 HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    {"message":"Dashboard starred!"}

## Unstar a dashboard

`DELETE /api/user/stars/dashboard/:dashboardId`

Deletes the starring of the given Dashboard for the actual user.

**Example Request**:

    DELETE /api/user/stars/dashboard/1 HTTP/1.1
    Accept: application/json
    Content-Type: application/json
    Authorization: Bearer eyJrIjoiT0tTcG1pUlY2RnVKZTFVaDFsNFZXdE9ZWmNrMkZYbk

**Example Response**:

    HTTP/1.1 200
    Content-Type: application/json

    {"message":"Dashboard unstarred"}

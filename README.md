# Keycloak Adapter for APPUiO Cloud

[![Build](https://img.shields.io/github/workflow/status/vshn/appuio-keycloak-adapter/Test)][build]
![Go version](https://img.shields.io/github/go-mod/go-version/vshn/appuio-keycloak-adapter)
[![Version](https://img.shields.io/github/v/release/vshn/appuio-keycloak-adapter)][releases]
[![Maintainability](https://img.shields.io/codeclimate/maintainability/vshn/appuio-keycloak-adapter)][codeclimate]
[![Coverage](https://img.shields.io/codeclimate/coverage/vshn/appuio-keycloak-adapter)][codeclimate]
[![GitHub downloads](https://img.shields.io/github/downloads/vshn/appuio-keycloak-adapter/total)][releases]

[build]: https://github.com/vshn/appuio-keycloak-adapter/actions?query=workflow%3ATest
[releases]: https://github.com/vshn/appuio-keycloak-adapter/releases
[codeclimate]: https://codeclimate.com/github/vshn/appuio-keycloak-adapter

The [APPUiO Control API](https://github.com/appuio/control-api) enables self-service for  [APPUiO Cloud](https://appuio.cloud).
One key part of this is to allow users to manage organizations and teams themselves.
However the APPUiO Control API does not require a specific identity provider (IdP), but has a plugin-like architecture and relies on Kubernetes controllers to interface with an IdP.

This project is such a controller that interfaces with Keycloak, the default IdP for APPUiO Cloud.

## Usage

```
Usage of ./appuio-keycloak-adapter:
  -keycloak-password string
      The password to log in to the Keycloak server.
  -keycloak-realm string
      The realm to sync the groups to.
  -keycloak-url https://keycloak.example.com
      The address of the Keycloak server (E.g. https://keycloak.example.com).
  -keycloak-username string
      The username to log in to the Keycloak server.

  -sync-schedule string
      A cron style schedule for the organization synchronization interval. (default "@every 5m")
  -sync-timeout duration
      The timeout for a single synchronization run. (default 10s)
  -sync-roles string
    	A comma separated list of cluster roles to bind to users when importing a new organization.
```

### Authenticating to Keycloak

A user with permissions to query for Keycloak groups as well as query and manage users must be available.
The following permissions must be associated to the user:

* Password must be set (Temporary option unselected) on the _Credentials_ tab
* On the _Role Mappings_ tab, select _realm-management_ next to the _Client Roles_ dropdown and then select **query-users**, **manage-users**, and **query-groups**.


### Organization Import

In addition to mirroring changes on `Organization` resources to Keycloak, this component will also periodically import any top-level Keycloak group as `Organizations`
It will however only create `Organization` resources and will never update them.
This import schedule is configured through the `sync-schedule` flag and the `ClusterRoles` specified in the `sync-roles` flag will be bound to every member of the Keycloak group at the time of the initial import.

## Development

### Run Locally

1. Start the local Control API: https://github.com/appuio/control-api/tree/master/local-env
1. Build controller `make build`
1. Setup Keycloak management user
1. Run controller against the local Control API.

**Make sure to connect to the local cluster.
If not configured otherwise the controller will connect to cluster defined in your current kubeconfig.**

```
./appuio-keycloak-adapter \
  --keycloak-url https://id.dev.appuio.cloud/ --keycloak-realm <your-dev-realm> \
  --keycloak-username <created-user> --keycloak-password <password>
```

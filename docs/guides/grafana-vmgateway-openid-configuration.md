# How to configure vmgateway for multi-tenant access using Grafana and OpenID Connect

Using Grafana with vmgateway is a great way to provide multi-tenant access to your metrics.
vmgateway provides a way to authenticate users using JWT tokens issued by an external identity provider.
Those tokens can include information about the user and the tenant they belong to, which can be used
to restrict access to metrics to only those that belong to the tenant.

## Prerequisites

* Identity service that can issue JWT tokens
* Grafana
* VictoriaMetrics single-node or cluster version
* vmgateway

## Configure identity service

The identity service must be able to issue JWT tokens with the following `vm_access` claim. Here is an example claims
object:

```json
{
  "vm_access": {
    "tenant_id": {
      "account_id": 0,
      "project_id": 0
    }
  }
}
```

See details about all supported options in the [vmgateway documentation](https://docs.victoriametrics.com/vmgateway.html#access-control).

### Configuration for Keycloak

1. Log in with admin credentials to your Keycloak instance
2. Go to `Clients` -> `Create`.
   Use `OpenID Connect` as `Client Type`.
   Specify `grafana` as `Client ID`.
   Click `Next`.
   <img src="grafana-vmgateway-openid-configuration/create-client-1.png" width="800">
3. Enable `Client authentication`.
   Enable `Authorization`.
   <img src="grafana-vmgateway-openid-configuration/create-client-2.png" width="800">
   Click `Next`.
4. Add Grafana URL as `Valid Redirect URIs`. For example, `http://localhost:3000/`.
   <img src="grafana-vmgateway-openid-configuration/create-client-3.png" width="800">
   Click `Save`.
5. Go to `Clients` -> `grafana` -> `Credentials`.
   <img src="grafana-vmgateway-openid-configuration/client-secret.png" width="800">
   Copy the value of `Client secret`. It will be used later in Grafana configuration.
6. Go to `Clients` -> `grafana` -> `Client scopes`.
   Click at `grafana-dedicated` -> `Add mapper`.
   <img src="grafana-vmgateway-openid-configuration/create-mapper-1.png" width="800">
   <img src="grafana-vmgateway-openid-configuration/create-mapper-2.png" width="800">
   Configure the mapper as follows
    - `Mapper Type` as `User Attribute`.
    - `Name` as `vm_access`.
    - `Token Claim Name` as `vm_access`.
    - `User Attribute` as `vm_access`.
    - `Claim JSON Type` as `JSON`.
      Enable `Add to ID token` and `Add to access token`.
      <img src="grafana-vmgateway-openid-configuration/create-mapper-3.png" width="800">
      Click `Save`.
7. Go to `Users` -> select user to configure claims -> `Attributes`.
   Specify `vm_access` as `Key`.
   Specify `{"tenant_id" : {"account_id": 0, "project_id": 0 }}` as `Value`.
   <img src="grafana-vmgateway-openid-configuration/user-attributes.png" width="800">
   Click `Save`.

## Configure grafana

It is required to configure Grafana to use OpenID Connect authentication and to forward the JWT token to vmgateway.

Example configuration for Grafana OpenID Connect authentication:
```ini
[auth.generic_oauth]
enabled = true
allow_sign_up = true
team_ids =
allowed_organizations =
name = keycloak
client_id = {CLIENT_ID_FROM_IDENTITY_PROVIDER}
client_secret = {SECRET_FROM_IDENTITY_PROVIDER}
scopes = openid profile email
auth_url = http://localhost:3001/realms/{KEYCLOACK_REALM}/protocol/openid-connect/auth
token_url = http://localhost:3001/realms/{KEYCLOACK_REALM}/protocol/openid-connect/token
api_url = http://localhost:3001/realms/{KEYCLOACK_REALM}/protocol/openid-connect/userinfo
```

Start Grafana. You should be able to log in using your identity provider.

Create a datasource in Grafana. For example, Prometheus datasource with the following URL `http://localhost:8431`.
Enable `Forward OAuth identity` flag.
<img src="grafana-vmgateway-openid-configuration/grafana-ds.png" width="800">

## Start vmgateway

Now starting vmgateway to enable authentication is as simple as adding the `-enable.auth=true` flag.
In order to enable multi-tenant access, you must also specify the `-clusterMode=true` flag.

```console
./bin/vmgateway -eula \
    -enable.auth=true \
    -clusterMode=true \
    -write.url=http://localhost:8480 \
    -read.url=http://localhost:8481
```

With this configuration vmgateway will use the `vm_access` claim from the JWT token to restrict access to metrics.
For example, if the JWT token contains the following `vm_access` claim:

```json
{
  "vm_access": {
    "tenant_id": {
      "account_id": 0,
      "project_id": 0
    }
  }
}
```

Then vmgateway will proxy request to an endpoint with the following path:

```console
http://localhost:8480/select/0:0/
```

This allows to restrict access to specific tenants without having to create separate datasources in Grafana,
or manually managing access at another proxy level. Moreover, using token claims such as `extra_labels`
or `extra_filters` allows to further restrict access to specific metrics dynamically by using Identity Provider's user information.

It is also possible to enable [JWT token signature verification](https://docs.victoriametrics.com/vmgateway.html#jwt-signature-verification) at
vmgateway.
For example, To do this by using OpenID Connect discovery endpoint y[grafana-vmgateway-openid-configuration.md](grafana-vmgateway-openid-configuration.md)ou need to specify
the `-auth.oidcDiscoveryEndpoints` flag. For example:

```console
./bin/vmgateway -eula \
    -enable.auth=true \
    -clusterMode=true \
    -write.url=http://localhost:8480 \
    -read.url=http://localhost:8481
    -auth.oidcDiscoveryEndpoints=http://localhost:3001/realms/master/.well-known/openid-configuration
```

Now vmgateway will print the following message on startup:

```console
2023-03-13T14:45:31.552Z        info    VictoriaMetrics/app/vmgateway/main.go:154  using 2 keys for JWT token signature verification
```

That means that vmgateway has successfully fetched the public keys from the OpenID Connect discovery endpoint.

It is also possible to provide the public keys directly via the `-auth.publicKeys` flag. See the [vmgateway documentation](https://docs.victoriametrics.com/vmgateway.html#jwt-signature-verification) for details.

## Use Grafana to query metrics

Now you can use Grafana to query metrics from the specified tenant.
Users with `vm_access` claim will be able to query metrics from the specified tenant.

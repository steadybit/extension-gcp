<img src="./logo.svg" height="130" align="right" alt="Google Cloud logo">

# Steadybit extension-gcp

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject faults into various Google Cloud / GCP services.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_gcp).

## Configuration

| Environment Variable                                   | Helm value                       | Meaning                                                                                                                                              | Required | Default                                        |
|--------------------------------------------------------|----------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|----------|------------------------------------------------|
| `STEADYBIT_EXTENSION_CREDENTIALS_KEYFILE_PATH`         | gcp.credentialsKeyfilePath       | To authorize using a JSON key file via location path (https://cloud.google.com/iam/docs/managing-service-account-keys)                               | false    | Tries to get a client with default google apis |
| `STEADYBIT_EXTENSION_PROJECT_ID`                       | gcp.projectID                    | The Google Cloud Project ID to be used                                                                                                               | true     |                                                |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_VM` | discovery.attributes.excludes.vm | List of Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                               | false    |                                                |

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

When installed as linux package this configuration is in`/etc/steadybit/extension-gcp`.

### Authorization configuration

Provide the credentials to authorize the extension to access the Google Cloud API. The extension supports two ways to provide the credentials:
Provide a JSON key file via the environment variable `STEADYBIT_EXTENSION_CREDENTIALS_KEYFILE_PATH` and mount it to the extension.
Or create a secret with the key `credentialsKeyfileJson` and provide the json there.

## Installation

### Using Docker

```sh
docker run \
  --rm \
  -p 8093 \
  --name steadybit-extension-gcp \
  -e STEADYBIT_EXTENSION_PROJECT_ID='YOUR_GCP_PROJECT_ID' \
  -e STEADYBIT_EXTENSION_CREDENTIALS_KEYFILE_JSON='{  "type": "service_account".......' \
  ghcr.io/steadybit/extension-gcp:latest
```

### Using Helm in Kubernetes

```sh
helm repo add steadybit-extension-gcp https://steadybit.github.io/extension-gcp
helm repo update
helm upgrade steadybit-extension-gcp \
    --install \
    --wait \
    --timeout 5m0s \
    --create-namespace \
    --namespace steadybit-extension \
    --set gcp.projectID=YOUR_GCP_PROJECT_ID \
    --set gcp.credentialsKeyfilePath=PATH_TO_JSON_FILE \
    steadybit-extension-gcp/steadybit-extension-gcp
```

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more information.

## Authorization scopes

### Discovery

To discover vm instances, the extension needs:

#### OAuth Scopes
one of the following OAuth scopes:

- `https://www.googleapis.com/auth/compute.readonly`
- `https://www.googleapis.com/auth/compute`
- `https://www.googleapis.com/auth/cloud-platform`

#### IAM Permissions
In addition to any permissions specified on the fields above, authorization requires one or more of the following IAM permissions:

- `compute.acceleratorTypes.list`

To find predefined roles that contain those permissions, see [Compute Engine IAM Roles](https://cloud.google.com/compute/docs/access/iam).


### Attack

To attack vm instances, the extension needs:

#### OAuth Scopes
one of the following OAuth scopes:

- `https://www.googleapis.com/auth/compute`
- `https://www.googleapis.com/auth/cloud-platform`

#### IAM Permissions

In addition to any permissions specified on the fields above, authorization requires one or more of the following IAM permissions:

- `compute.instances.reset`
- `compute.instances.stop`
- `compute.instances.suspend`
- `compute.instances.delete`

To find predefined roles that contain those permissions, see [Compute Engine IAM Roles](https://cloud.google.com/compute/docs/access/iam).


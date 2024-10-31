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

### Kubernetes

Detailed information about agent and extension installation in kubernetes can also be found in
our [documentation](https://docs.steadybit.com/install-and-configure/install-agent/install-on-kubernetes).

#### Recommended (via agent helm chart)

All extensions provide a helm chart that is also integrated in the
[helm-chart](https://github.com/steadybit/helm-charts/tree/main/charts/steadybit-agent) of the agent.

You must provide additional values to activate this extension.

```
--set extension-gcp.enabled=true \
--set extension-gcp.gcp.projectID=YOUR_GCP_PROJECT_ID \
--set extension-gcp.gcp.credentialsKeyfilePath=PATH_TO_JSON_FILE \
```

Additional configuration options can be found in
the [helm-chart](https://github.com/steadybit/extension-gcp/blob/main/charts/steadybit-extension-gcp/values.yaml) of the
extension.

#### Alternative (via own helm chart)

If you need more control, you can install the extension via its
dedicated [helm-chart](https://github.com/steadybit/extension-gcp/blob/main/charts/steadybit-extension-gcp).

```bash
helm repo add steadybit-extension-gcp https://steadybit.github.io/extension-gcp
helm repo update
helm upgrade steadybit-extension-gcp \
    --install \
    --wait \
    --timeout 5m0s \
    --create-namespace \
    --namespace steadybit-agent \
    --set gcp.projectID=YOUR_GCP_PROJECT_ID \
    --set gcp.credentialsKeyfilePath=PATH_TO_JSON_FILE \
    steadybit-extension-gcp/steadybit-extension-gcp
```

### Linux Package

Please use
our [agent-linux.sh script](https://docs.steadybit.com/install-and-configure/install-agent/install-on-linux-hosts)
to install the extension on your Linux machine. The script will download the latest version of the extension and install
it using the package manager.

After installing, configure the extension by editing `/etc/steadybit/extension-gcp` and then restart the service.

## Extension registration

Make sure that the extension is registered with the agent. In most cases this is done automatically. Please refer to
the [documentation](https://docs.steadybit.com/install-and-configure/install-agent/extension-discovery) for more
information about extension registration and how to verify.

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

- `compute.instances.list`

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


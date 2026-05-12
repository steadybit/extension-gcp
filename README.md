<img src="./logo.svg" height="130" align="right" alt="Google Cloud logo">

# Steadybit extension-gcp

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject faults into various Google Cloud / GCP services.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_gcp).

## Configuration

| Environment Variable                                   | Helm value                       | Meaning                                                                                                                                                                                               | Required | Default                                        |
|--------------------------------------------------------|----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|------------------------------------------------|
| `STEADYBIT_EXTENSION_CREDENTIALS_KEYFILE_PATH`         | gcp.credentialsKeyfilePath       | To authorize using a JSON key file via location path (https://cloud.google.com/iam/docs/managing-service-account-keys)                                                                                | false    | Tries to get a client with default google apis |
| `STEADYBIT_EXTENSION_PROJECT_ID`                       | gcp.projectID                    | Legacy single-project configuration. Kept for backward compatibility. Mutually exclusive with `STEADYBIT_EXTENSION_PROJECT_IDS` and `STEADYBIT_EXTENSION_PROJECTS_ADVANCED`.                          | false    |                                                |
| `STEADYBIT_EXTENSION_PROJECT_IDS`                      | gcp.projectIDs                   | Comma-separated list of GCP project IDs to discover. All projects are accessed with the same credentials (ADC or `CREDENTIALS_KEYFILE_PATH`).                                                         | false    |                                                |
| `STEADYBIT_EXTENSION_PROJECTS_ADVANCED`                | gcp.projectsAdvanced             | JSON array configuring per-project service-account impersonation, e.g. `[{"projectId":"proj-a","impersonateServiceAccount":"sa@proj-a.iam.gserviceaccount.com"}]`.                                    | false    |                                                |
| `STEADYBIT_EXTENSION_WORKER_THREADS`                   | gcp.workerThreads                | Number of goroutines used to fan discovery across configured projects.                                                                                                                                | false    | 1                                              |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_VM` | discovery.attributes.excludes.vm | List of Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*"                                                                                | false    |                                                |

Exactly one of `STEADYBIT_EXTENSION_PROJECT_ID`, `STEADYBIT_EXTENSION_PROJECT_IDS`, or `STEADYBIT_EXTENSION_PROJECTS_ADVANCED` must be set; setting more than one fails startup.

### Opt-in discoveries

These discoveries are disabled by default. Set the matching env var (or Helm `discovery.enable.*` flag) to `true` to enable each module. Per-module `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_*` lists work the same way as the VM one.

| Module                            | Env var                                                  | Helm flag                                  |
|-----------------------------------|----------------------------------------------------------|--------------------------------------------|
| GKE cluster                       | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_GKE_CLUSTER`       | `discovery.enable.gkeCluster`              |
| GKE node pool (+ terminate-instances attack) | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_GKE_NODE_POOL`     | `discovery.enable.gkeNodePool`             |
| Managed Instance Group (+ delete-instances attack) | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_MIG`               | `discovery.enable.mig`                     |
| Cloud NAT (+ disassociate-subnet attack) | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_CLOUD_NAT`          | `discovery.enable.cloudNat`                |
| Persistent Disk                   | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_PERSISTENT_DISK`   | `discovery.enable.persistentDisk`          |
| Cloud SQL (+ failover attack)     | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_CLOUD_SQL`         | `discovery.enable.cloudSql`                |
| Spanner instance                  | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_SPANNER`           | `discovery.enable.spanner`                 |
| Pub/Sub topic                     | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_PUB_SUB_TOPIC`     | `discovery.enable.pubSubTopic`             |
| Pub/Sub subscription              | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_PUB_SUB_SUBSCRIPTION` | `discovery.enable.pubSubSubscription`   |
| Memorystore Redis (+ failover attack) | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_MEMORYSTORE_REDIS` | `discovery.enable.memorystoreRedis`        |
| Cloud Run service                 | `STEADYBIT_EXTENSION_DISCOVERY_ENABLE_CLOUD_RUN`         | `discovery.enable.cloudRun`                |

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

When installed as linux package this configuration is in`/etc/steadybit/extension-gcp`.

### Authorization configuration

Provide the credentials to authorize the extension to access the Google Cloud API. The extension supports two ways to provide the credentials:
Provide a JSON key file via the environment variable `STEADYBIT_EXTENSION_CREDENTIALS_KEYFILE_PATH` and mount it to the extension.
Or create a secret with the key `credentialsKeyfileJson` and provide the json there.

### Multi-project configuration

The extension can discover resources across multiple GCP projects. Two modes are supported:

#### Shared credentials (simple)

List the projects in `STEADYBIT_EXTENSION_PROJECT_IDS` / `gcp.projectIDs`. The same identity (ADC or keyfile) is used to call every project, so that identity must hold the required permissions in each project.

```
--set gcp.projectIDs="proj-a,proj-b,proj-c"
```

#### Per-project service-account impersonation (advanced)

Use `STEADYBIT_EXTENSION_PROJECTS_ADVANCED` / `gcp.projectsAdvanced` to define a dedicated service account per project. At runtime the extension's base identity exchanges tokens via the IAM Credentials API (`iam.serviceAccounts.getAccessToken`) to act as each target service account. This is the recommended pattern for environments that isolate permissions per project.

```yaml
gcp:
  projectsAdvanced: |
    [
      {"projectId":"proj-a","impersonateServiceAccount":"extension@proj-a.iam.gserviceaccount.com"},
      {"projectId":"proj-b","impersonateServiceAccount":"extension@proj-b.iam.gserviceaccount.com"}
    ]
```

Prerequisites for impersonation:

1. Each target project has a dedicated service account (e.g. `extension@proj-a.iam.gserviceaccount.com`) with the IAM roles it needs to perform the configured attacks.
2. The identity the extension runs as (its base ADC or keyfile service account) has the `roles/iam.serviceAccountTokenCreator` role on every target service account. See [Service account impersonation](https://cloud.google.com/iam/docs/service-account-impersonation).

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
the [documentation](https://docs.steadybit.com/install-and-configure/install-agent/extension-registration) for more
information about extension registration and how to verify.

## IAM Permissions

### Discovery

To discover vm instances, the extension needs the following IAM permissions:

- `compute.instances.list`

For the opt-in discoveries, the extension additionally needs:

- GKE cluster / node pool: `container.clusters.list`, `container.clusters.get`, `container.nodePools.list`
- MIG: `compute.instanceGroupManagers.list`, `compute.regionInstanceGroupManagers.list`
- Cloud NAT: `compute.routers.list`
- Persistent Disk: `compute.disks.list`, `compute.regionDisks.list`
- Cloud SQL: `cloudsql.instances.list`
- Spanner: `spanner.instances.list`
- Pub/Sub: `pubsub.topics.list`, `pubsub.subscriptions.list`
- Memorystore Redis: `redis.instances.list`
- Cloud Run: `run.services.list`

### Attack

To attack vm instances, the extension needs the following IAM permissions:

- `compute.instances.reset`
- `compute.instances.stop`
- `compute.instances.suspend`
- `compute.instances.delete`
- `compute.instances.start`

For the opt-in attacks, the extension additionally needs:

- GKE node pool terminate-instances: `compute.instances.delete` on the underlying MIG (`compute.instanceGroupManagers.delete*Instances`), `compute.instanceGroupManagers.listManagedInstances`
- MIG delete-instances: `compute.instanceGroupManagers.deleteInstances` (and `compute.regionInstanceGroupManagers.deleteInstances` for regional MIGs)
- Cloud NAT disassociate: `compute.routers.get`, `compute.routers.patch`
- Cloud SQL failover: `cloudsql.instances.failover`
- Memorystore Redis failover: `redis.instances.failover`

### Create Role and ServiceAccount

1. Create a service role "steadybit-extension-gcp" with the following permissions:

- `compute.instances.list`
- `compute.instances.reset`
- `compute.instances.stop`
- `compute.instances.suspend`
- `compute.instances.delete`
- `compute.instances.start`

2. Create a service account using the role "steadybit-extension-gcp".
3. Create an access key for that service account and download the JSON key to key.json
4. Create a kubernetes secret with the key.json file:
```bash
kubectl create secret generic extension-gcp -n steadybit-agent \
    --from-file=credentialsKeyfileJson=./key.json
```

5. Apply the helm chart while refenrencing the created secret


## Version and Revision

The version and revision of the extension:
- are printed during the startup of the extension
- are added as a Docker label to the image
- are available via the `version.txt`/`revision.txt` files in the root of the image

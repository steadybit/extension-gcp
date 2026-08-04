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

### Attack safety

The five new attacks are not all reversible. Read this before turning them on in production:

| Attack | Reversibility | What actually happens |
|--------|---------------|------------------------|
| GKE node pool: terminate-instances | **Destructive, self-healing.** Deleted instances are gone forever; the MIG creates new replacements per its scaling/heal policies. Recovery time depends on cluster-autoscaler and surge config — a misconfigured pool may stay undersized indefinitely. Percentages above 50% require an explicit confirmation flag. |
| MIG: delete-instances | **Destructive, self-healing.** Same model as the GKE attack: the MIG creates new replacements. A MIG without autoscaling stays undersized until an operator intervenes. Percentages above 50% require explicit confirmation. |
| Cloud NAT: disassociate subnetworks | **Truly reversible.** Original subnetwork list is captured at Prepare and restored at Stop. Re-fetches the router on every patch so concurrent edits to other NATs on the same router are preserved. If Stop never runs (agent crash, abandoned experiment), the NAT stays disassociated until an operator restores it. |
| Cloud SQL: failover | **Not reversible.** Promotes the REGIONAL standby to primary; Cloud SQL rebuilds a new HA standby behind it. Exercises the same code path as a real zonal outage. Gated on `availability-type=REGIONAL`. |
| Memorystore Redis: failover | **Not reversible.** Promotes the standby for STANDARD_HA instances; exercises the same code path as a real primary-node outage. `FORCE_DATA_LOSS` may drop in-flight writes that have not yet been replicated. Gated on `tier=STANDARD_HA`. |

Beyond the settings above, this extension supports the configuration common to all Steadybit
extensions:

- [extension-kit](https://github.com/steadybit/extension-kit#environment-variables) — HTTP and
  health ports, TLS and mutual TLS, unix domain socket, logging, and pprof.
- [Target Filtering](https://github.com/steadybit/discovery-kit/blob/main/docs/target-filtering.md) —
  stop the extension reporting targets you do not want.
- [Group Matching](https://github.com/steadybit/discovery-kit/blob/main/docs/target-enrichment.md#group-matching) —
  tag discovered targets with a group, so enrichment rules only match within it.

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

### Enable the GCP APIs

Each module the extension discovers or attacks against calls a specific GCP API. Enable them once per project *before* granting IAM roles — otherwise IAM alone won't help and calls fail with `SERVICE_DISABLED`. Enable only the ones you actually enable in `discovery.enable.*`:

```bash
PROJECT_ID=your-project

# Always required (VM discovery + state action)
gcloud services enable compute.googleapis.com --project="$PROJECT_ID"

# Per opt-in module — enable only what you turn on
gcloud services enable container.googleapis.com     --project="$PROJECT_ID"  # GKE cluster + node pool
gcloud services enable sqladmin.googleapis.com      --project="$PROJECT_ID"  # Cloud SQL
gcloud services enable spanner.googleapis.com       --project="$PROJECT_ID"  # Spanner
gcloud services enable pubsub.googleapis.com        --project="$PROJECT_ID"  # Pub/Sub topic + subscription
gcloud services enable redis.googleapis.com         --project="$PROJECT_ID"  # Memorystore Redis
gcloud services enable run.googleapis.com           --project="$PROJECT_ID"  # Cloud Run
# MIG, Cloud NAT, Persistent Disk are all under compute.googleapis.com — no extra enablement needed.
```

Enabling an API only propagates within a few minutes; discovery may keep returning `SERVICE_DISABLED` briefly after the flag flips on.

### Fine-grained permissions

If you build a custom role, these are the exact permissions the extension calls:

**Discovery (always required for VM)**
- `compute.instances.list`

**Discovery (opt-in modules — grant only what you enable)**
- GKE cluster / node pool: `container.clusters.list`, `container.clusters.get`, `container.nodePools.list`
- MIG: `compute.instanceGroupManagers.list`, `compute.regionInstanceGroupManagers.list`
- Cloud NAT: `compute.routers.list`
- Persistent Disk: `compute.disks.list`, `compute.regionDisks.list`
- Cloud SQL: `cloudsql.instances.list`
- Spanner: `spanner.instances.list`
- Pub/Sub: `pubsub.topics.list`, `pubsub.subscriptions.list`
- Memorystore Redis: `redis.instances.list`
- Cloud Run: `run.services.list`

**Attacks (always required for VM state)**
- `compute.instances.reset`, `compute.instances.stop`, `compute.instances.suspend`, `compute.instances.delete`, `compute.instances.start`

**Attacks (opt-in modules)**
- GKE node pool terminate-instances: `compute.instanceGroupManagers.listManagedInstances`, `compute.instanceGroupManagers.deleteInstances`
- MIG delete-instances: `compute.instanceGroupManagers.deleteInstances` (and `compute.regionInstanceGroupManagers.deleteInstances` for regional MIGs)
- Cloud NAT disassociate: `compute.routers.get`, `compute.routers.patch`
- Cloud SQL failover: `cloudsql.instances.failover`
- Memorystore Redis failover: `redis.instances.failover`

### Suggested pre-defined roles

Composing a custom role is fiddly. Most operators just bind Google's pre-defined roles — each one below covers both the discovery reads and the corresponding attack's mutations for that module.

| Module | Pre-defined role | Notes |
|---|---|---|
| VM (state action) + MIG (delete-instances) + GKE node pool (terminate-instances) | `roles/compute.instanceAdmin.v1` | Covers `compute.instances.*` + `compute.instanceGroupManagers.deleteInstances`. |
| Any Compute discovery (routers, MIGs, disks) | `roles/compute.viewer` | Combine with `instanceAdmin.v1` above; viewer is broader for reads. |
| Cloud NAT disassociate | `roles/compute.networkAdmin` | Grants `compute.routers.patch`. |
| GKE cluster + node pool | `roles/container.developer` | Discovery reads. Terminate-instances uses `compute.instanceAdmin.v1` above (nodes are Compute-side). |
| Cloud SQL discovery + failover | `roles/cloudsql.admin` | Downgrade to `roles/cloudsql.viewer` if you don't need the failover attack. |
| Memorystore Redis discovery + failover | `roles/redis.admin` | Downgrade to `roles/redis.viewer` if you don't need the failover attack. |
| Pub/Sub discovery | `roles/pubsub.viewer` | No attacks in this extension. |
| Cloud Run discovery | `roles/run.viewer` | No attacks in this extension. |
| Spanner discovery | `roles/spanner.viewer` | No attacks in this extension. |

If you use `STEADYBIT_EXTENSION_PROJECTS_ADVANCED` (per-project service-account impersonation), also grant `roles/iam.serviceAccountTokenCreator` on each target service account to the base identity the extension runs as.

### Create Role and ServiceAccount

1. Enable the GCP APIs listed above for every project you configure in `gcp.projectID` / `gcp.projectIDs` / `gcp.projectsAdvanced`.
2. Create a service account `steadybit-extension-gcp@<project>.iam.gserviceaccount.com`.
3. Bind the pre-defined roles from the table above (or a custom role built from the fine-grained permissions) to that service account on every target project.
4. Create an access key for the service account and download the JSON key to `key.json`.
5. Create a Kubernetes secret with the key file:

   ```bash
   kubectl create secret generic extension-gcp -n steadybit-agent \
       --from-file=credentialsKeyfileJson=./key.json
   ```

6. Install the Helm chart referencing the secret; opt each module in via `discovery.enable.*` in the values file.


## Version and Revision

The version and revision of the extension:
- are printed during the startup of the extension
- are added as a Docker label to the image
- are available via the `version.txt`/`revision.txt` files in the root of the image

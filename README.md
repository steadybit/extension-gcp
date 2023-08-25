<img src="./logo.svg" height="130" align="right" alt="Google Cloud logo">

# Steadybit extension-gcp

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject faults into various Google Cloud / GCP services.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_gcp).

## Configuration

| Environment Variable                       | Helm value | Meaning                                                                                              | Required | Default |
|--------------------------------------------|------------|------------------------------------------------------------------------------------------------------|----------|---------|
| `STEADYBIT_EXTENSION_CREDENTIALS_KEY_FILE` |            | To authorize using a JSON key file (https://cloud.google.com/iam/docs/managing-service-account-keys) | false    |         |

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

## Installation

### Using Docker

```sh
docker run \
  --rm \
  -p 8080 \
  --name steadybit-extension-gcp \
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
    steadybit-extension-gcp/steadybit-extension-gcp
```

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to
the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more information.

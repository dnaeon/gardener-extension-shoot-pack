# PACKAGE format

Each pack is described by a `PACKAGE` spec. Package specs reside in the [specs directory](../specs/)
of the repo.

The `PACKAGE` spec is a recipe, which provides reproducible instructions on how
to generate a set of Kubernetes resources.

This set of resources could represent an upstream operator
like [CloudNativePG Operator](../specs/cloudnativepg), [Prometheus Operator](../specs/prometheus-operator),
[Valkey Operator](../specs/valkey-operator), etc., or anything else.

The final set of resources is called a package, or _pack_ for short.

The `gardener-extension-shoot-pack` extension does not care or know how a pack
was built -- whether these resources came from a Helm chart, a kustomize
overlay, etc.

Since the resources that make up a pack are plain Kubernetes resources, the
packs allow us to consume upstream Kubernetes projects (e.g. operators) and
distribute them to Gardener shoot clusters in a generic and agnostic way.

## PACKAGE spec format

Lets use the `PACKAGE` spec for [CloudNativePG Operator](../specs/cloudnativepg)
as an example and describe it. This is what the spec looks like.

``` shell
NAME=cnpg-operator
VERSION=v1.28.1
DESCRIPTION="CloudNativePG Operator"
CHART_VERSION=0.27.1

package() {
  local release_name="extension"

  # Render the upstream chart, clean it up, and slice it up.
  ${HELM} repo add cnpg https://cloudnative-pg.github.io/charts
  ${HELM} repo update cnpg
  ${HELM} template \
          --version ${CHART_VERSION} \
          --release-name ${release_name} \
          --namespace ${PACK_NAMESPACE} \
          --values ${SRC_DIR}/values.yaml \
          cnpg/cloudnative-pg | \
    grep -v '^#' | \
    ${KUBECTL_SLICE} --exclude-kind Namespace -f - --output-dir ${PACK_DIR}/
}
```

Each `PACKAGE` spec must define the following variables.

| Variable      | Description                   |
|---------------|-------------------------------|
| `NAME`        | Name of the pack              |
| `VERSION`     | Pack version                  |
| `DESCRIPTION` | Short description of the pack |

Additionally, a `PACKAGE` spec must specify a `package()` function, and
optionally a `package_test()` function.

| Function         | Description                                                                      |
|------------------|----------------------------------------------------------------------------------|
| `package()`      | Provides the instructions for producing the pack resources (required)            |
| `package_test()` | Provides instructions for verifying and testing of the pack resources (optional) |

The `package()` function is invoked in order to [build the pack](../scripts/pack-build.sh).
Any Kubernetes resource produced by this function should be
persisted in the `${PACK_DIR}` directory. A valid Kubernetes resource should have the
`.yaml` file extension.

The `package_test()` function, if defined, will be invoked during
[pack verification](../scripts/pack-verify.sh).

In the example `PACKAGE` spec for CloudNativePG above the `package()` function
is used to:

1. Configure the upstream Helm repo for CloudNativePG
2. Render the Helm chart with [custom values](../specs/cloudnativepg/values.yaml)
3. Slice the resulting bundle of resources into individual Kubernetes resources
4. Persist the set of resources into the `${PACK_DIR}` directory

You should also notice that in the pack spec above Helm is being referenced via
the `${HELM}` env var, similar to how
[kubectl-slice](https://github.com/patrickdappollonio/kubectl-slice) is
referenced via the `${KUBECTL_SLICE}` var. These variables are part of the
environment, which is automatically configured during pack building.

This allows us to package resources in a reproducible way, because the same
recipe could be executed by you, me, a pipeline, or anything else, without
having to rely on a pre-installed list of tools.

## Pack Environment

During pack [build](../scripts/pack-build.sh) and
[verification](../scripts/pack-verify.sh) certain variables are automatically
configured, which can/should be used during the process of building and
verification of a pack.

The following table provides a short summary of the various env vars, which are
automatically configured during pack build and verification.

| Name             | Description                                               |
|------------------|-----------------------------------------------------------|
| `SRC_DIR`        | Points to the source directory of a `PACKAGE` spec        |
| `PACK_DIR`       | Points to the target directory for pack resources         |
| `PACK_NAMESPACE` | The namespace in which pack resources should be installed |

For more details on each of these variables, please refer to their respective
documentation in the sections below.

In addition to the env vars above, the following tools are also available via
their respective env vars.

| Name            | Description                                                                   |
|-----------------|-------------------------------------------------------------------------------|
| `YQ`            | The [yq](https://github.com/mikefarah/yq) CLI                                 |
| `KUSTOMIZE`     | The [kustomize](https://github.com/kubernetes-sigs/kustomize) CLI             |
| `HELM`          | The [Helm](https://github.com/helm/helm) CLI                                  |
| `KUBECTL_SLICE` | The [kubectl-slice](https://github.com/patrickdappollonio/kubectl-slice) tool |
| `KUBECONFORM`   | The [kubeconform](https://github.com/yannh/kubeconform) tool                  |

Packs can refer to these tools and use them during pack build or verification,
as shown in the example spec for CloudNativePG.

### SRC_DIR

The `SRC_DIR` var points to the source directory of the pack, which is the directory
that contains the `PACKAGE` spec. Packs can use this variable in order to refer
to additional files, which may be required for building or verifying a pack, as
is the example with CloudNativePG, where we provide [custom values](../specs/cloudnativepg/values.yaml)
to the upstream Helm chart.

### PACK_DIR

The `PACK_DIR` var points to the directory, where the resources built by a pack
should be saved to.

The set of resources persisted in `PACK_DIR` during pack build is what becomes
the final pack.

### PACK_NAMESPACE

The `PACK_NAMESPACE` var specifies the Kubernetes namespace in which the pack
resources would be installed to.

Due to a limitation in the Gardener Resource Manager this variable is always
`kube-system`, which means that the resources provided by a pack will be
installed into the `kube-system` namespace of the shoot cluster.

For more details about the limitation in Gardener, please refer to the following
links.

- https://gardener.cloud/docs/getting-started/ca-components/#managedresources
- https://github.com/gardener/gardener/issues/14342
- https://github.com/gardener/gardener/pull/14335

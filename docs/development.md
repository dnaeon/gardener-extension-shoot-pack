# Development

This document provides information to get you started with the development of
the extension.

In order to build a binary of the extension, you can use the following command.

``` shell
make build
```

The resulting binary can be found in `bin/extension`.

In order to build a Docker image of the extension, you can use the following
command.

``` shell
make docker-build
```

Run the following command to get usage info about the available Makefile
targets.

``` shell
make help
```

For local development of the `gardener-extension-shoot-pack` it is recommended that
you setup a [development Gardener environment](https://gardener.cloud/docs/gardener/local_setup/).

Please refer to the next sections for more information about deploying and
testing the extension in a Gardener development environment.

## Development Environment without Gardener Operator

The following documents describe how to create a Gardener development
environment locally. Please make sure to read them in order to familiarize
yourself with the setup, and also to install any prerequisites.

- [Gardener: Local setup requirements](https://gardener.cloud/docs/gardener/local_setup/)
- [Gardener: Getting Started Locally](https://gardener.cloud/docs/gardener/deployment/getting_started_locally/)

The steps from this section describe how to deploy and develop the extension
against a local development environment, without the
[Gardener Operator](https://gardener.cloud/docs/gardener/concepts/operator/).

In summary, these are the steps you need to follow in order to start a local
development Gardener environment, however, please make sure that you read the
documents above for additional details.

``` shell
make kind-up gardener-up
```

Before you continue with the next steps, make sure that you configure your
`KUBECONFIG` to point to the kubeconfig file created by Gardener for you.

This file will be located in the
`/path/to/gardener/example/gardener-local/kind/local/kubeconfig` path after
creating the dev environment.

``` shell
export KUBECONFIG=/path/to/gardener/example/gardener-local/kind/local/kubeconfig
```

You can use the following command in order to load the OCI image to the nodes of
your local Gardener cluster, which is running in
[kind](https://kind.sigs.k8s.io/).

``` shell
make kind-load-image
```

The Helm charts, which are used by the
[gardenlet](https://gardener.cloud/docs/gardener/concepts/gardenlet/) for
deploying the extension can be pushed to the local OCI registry using the
following command.

``` shell
make helm-load-chart
```

In the [examples/dev-setup](./examples/dev-setup) directory you can find
[kustomize](https://kustomize.io/]) resources, which can be used to create the
`ControllerDeployment` and `ControllerRegistration` resources.

For more information about `ControllerDeployment` and `ControllerRegistration`
resources, please make sure to check the
[Registering Extension Controllers](https://gardener.cloud/docs/gardener/extensions/registration/)
documentation.

The `deploy` target takes care of deploying your extension in a local Gardener
environment. It does the following.

1. Builds a Docker image of the extension
2. Loads the image into the `kind` cluster nodes
3. Packages the Helm charts and pushes them to the local registry
4. Deploys the `ControllerDeployment` and `ControllerRegistration` resources

``` shell
make deploy
```

Verify that we have successfully created the `ControllerDeployment` and
`ControllerRegistration` resources.

``` shell
$ kubectl get controllerregistrations,controllerdeployments gardener-extension-shoot-pack
NAME                                                                    RESOURCES           AGE
controllerregistration.core.gardener.cloud/gardener-extension-shoot-pack   Extension/pack   40s

NAME                                                                  AGE
controllerdeployment.core.gardener.cloud/gardener-extension-shoot-pack   40s
```

Finally, we can create an example shoot with our extension enabled. The
[examples/shoot.yaml](./examples/shoot.yaml) file provides a ready-to-use shoot
manifest with the extension enabled and configured.

``` shell
kubectl apply -f examples/shoot.yaml
```

Once we create the shoot cluster, `gardenlet` will start deploying our
`gardener-extension-shoot-pack`, since it is required by our shoot.

Verify that the extension has been successfully installed by checking the
corresponding `ControllerInstallation` resource.

``` shell
$ kubectl get controllerinstallations.core.gardener.cloud
NAME                               REGISTRATION                 SEED    VALID   INSTALLED   HEALTHY   PROGRESSING   AGE
gardener-extension-shoot-pack-tktwt   gardener-extension-shoot-pack   local   True    True        True      False         103s
```

After your shoot cluster has been successfully created and reconciled, verify
that the extension is healthy.

``` shell
$ kubectl --namespace shoot--local--local get extensions
NAME      TYPE      STATUS      AGE
pack   pack   Succeeded   85m
```

In order to trigger reconciliation of the extension you can annotate the
extension resource.

``` shell
kubectl --namespace shoot--local--local annotate extensions pack gardener.cloud/operation=reconcile
```

## Development Environment with Gardener Operator

The extension can also be deployed via the
[Gardener Operator](https://gardener.cloud/docs/gardener/concepts/operator/).

In order to start a local development environment with the Gardener Operator,
please refer to the following documentations.

- [Gardener Operator](https://gardener.cloud/docs/gardener/concepts/operator/)
- [Gardener: Local setup with gardener-operator](https://gardener.cloud/docs/gardener/deployment/getting_started_locally/#alternative-way-to-set-up-garden-and-seed-leveraging-gardener-operator)

In summary, these are the steps you need to follow in order to start a local
development environment with the [Gardener Operator](https://gardener.cloud/docs/gardener/concepts/operator/),
however, please make sure that you read the documents above for additional details.

``` shell
make kind-multi-zone-up operator-up operator-seed-up
```

Before you continue with the next steps, make sure that you configure your
`KUBECONFIG` to point to the kubeconfig file of the cluster, which runs the
Gardener Operator.

There will be two kubeconfig files created for you, after the dev environment
has been created.

| Path                                                                | Description                                            |
|---------------------------------------------------------------------|--------------------------------------------------------|
| `/path/to/gardener/dev-setup/kubeconfigs/runtime/kubeconfig`        | The _runtime_ cluster (`gardener-operator` runs in it) |
| `/path/to/gardener/dev-setup/kubeconfigs/virtual-garden/kubeconfig` | The _virtual_ garden cluster                           |

Throughout this document we will refer to the kubeconfigs for _runtime_ and
_virtual_ clusters as `$KUBECONFIG_RUNTIME` and `$KUBECONFIG_VIRTUAL`
respectively.

Before deploying the extension we need to target the _runtime_ cluster, since
this is where the extension resources for `gardener-operator` reside.

``` shell
export KUBECONFIG=$KUBECONFIG_RUNTIME
```

In order to deploy the extension, execute the following command.

``` shell
make deploy-operator
```

The `deploy-operator` target takes care of the following.

1. Builds a Docker image of the extension
2. Loads the image into the `kind` cluster nodes
3. Packages the Helm charts and pushes them to the local registry
4. Deploys the `Extension` (from group `operator.gardener.cloud/v1alpha1`) to
   the _runtime_ cluster

Verify that we have successfully created the
`Extension` (from group `operator.gardener.cloud/v1alpha1`) resource.

``` shell
$ kubectl --kubeconfig $KUBECONFIG_RUNTIME get extop gardener-extension-shoot-pack
NAME                         INSTALLED   REQUIRED RUNTIME   REQUIRED VIRTUAL   AGE
gardener-extension-shoot-pack   True        False              False              85s
```

Verify that the respective `ControllerRegistration` and `ControllerDeployment`
resources have been created by the `gardener-operator` in the _virtual_ garden
cluster.

``` shell
> kubectl --kubeconfig $KUBECONFIG_VIRTUAL get controllerregistrations,controllerdeployments gardener-extension-shoot-pack
NAME                                                                    RESOURCES           AGE
controllerregistration.core.gardener.cloud/gardener-extension-shoot-pack   Extension/pack  3m50s

NAME                                                                  AGE
controllerdeployment.core.gardener.cloud/gardener-extension-shoot-pack   3m50s
```

Now we can create an example shoot with our extension enabled. The
[examples/shoot.yaml](./examples/shoot.yaml) file provides a ready-to-use shoot
manifest, which we will use.

``` shell
kubectl --kubeconfig $KUBECONFIG_VIRTUAL apply -f examples/shoot.yaml
```

Once we create the shoot cluster, `gardenlet` will start deploying our
`gardener-extension-shoot-pack`, since it is required by our shoot.

Verify that the extension has been successfully installed by checking the
corresponding `ControllerInstallation` resource for our extension.

``` shell
$ kubectl --kubeconfig $KUBECONFIG_VIRTUAL get controllerinstallations.core.gardener.cloud
NAME                               REGISTRATION                 SEED    VALID   INSTALLED   HEALTHY   PROGRESSING   AGE
gardener-extension-shoot-pack-ng4r8   gardener-extension-shoot-pack   local   True    True        True      False         2m9s
```

After your shoot cluster has been successfully created and reconciled, verify
that the extension is healthy.

``` shell
$ kubectl --kubeconfig $KUBECONFIG_RUNTIME --namespace shoot--local--local get extensions
NAME      TYPE      STATUS      AGE
pack   pack   Succeeded   2m37s
```

In order to trigger reconciliation of the extension you can annotate the
extension resource.

``` shell
kubectl --kubeconfig $KUBECONFIG_RUNTIME --namespace shoot--local--local annotate extensions pack gardener.cloud/operation=reconcile
```

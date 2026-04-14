# Customizing Pack Resources

Each resource provided by a [pack](./packs.md) may be customized by shoot
cluster owners by providing a patch.

A patch is a snippet of the Kubernetes resource spec, which contains the changes
we want to apply.

Patches are described via [Gardener Referenced Resources](https://gardener.cloud/docs/gardener/extensions/referenced-resources/#referenced-resources),
which exist as secrets in the project namespace of the shoot cluster.
This allows a single patch to be re-used by multiple shoot clusters within the
same project.

Let's use the [CloudNativePG pack](../specs/cloudnativepg) as an example on how
to make a change to the resources provided by the pack.

The CloudNativePG pack provides a `Deployment` for the CloudNativePG operator,
which is specified in the
[deployment-extension-cloudnative-pg.yaml](../pkg/assets/packs/cnpg-operator/v1.28.1/deployment-extension-cloudnative-pg.yaml)
pack resource.

This example patch adds additional labels, env vars, and sets resource requests
to the CloudNativePG deployment.

``` yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: extension-cloudnative-pg
  namespace: kube-system
spec:
  template:
    # Add some new labels
    metadata:
      labels:
        label-foo: value-bar
        label-bar: value-foo
    spec:
      containers:
      - name: manager
        env:
        # Add a custom env var
        - name: MY_CUSTOM_ENV_VAR
          value: MY_CUSTOM_VALUE
        # Add custom resources
        resources:
          requests:
            cpu: 10m
            memory: 64Mi
```

Save this patch as `pack-cnpg-deployment-patch.yaml`, because we will use it a
bit later.

In order to use this patch we need to create a secret in the project namespace
of our shoot cluster. The example command below creates the secret in the
project namespace for local shoots.

Make sure to adjust the project namespace, in case you are creating the secret
elsewhere.

``` shell
kubectl \
  --namespace garden-local \
  create secret generic pack-cnpg-deployment-patch \
  --from-file=pack-cnpg-deployment-patch.yaml
```

Now that we have the secret in the project namespace, we can refer to it in the
shoot spec via a [Referenced Resource](https://gardener.cloud/docs/gardener/extensions/referenced-resources/#referenced-resources).

In order to enable this patch for the `cnpg-operator@v1.28.1` pack we can use
the following extension config in our shoot spec.

``` yaml
...

spec:
  extensions:
    - type: shoot-pack
      providerConfig:
        apiVersion: pack.extensions.gardener.cloud/v1alpha1
        kind: PackConfig
        spec:
          packs:
          - name: cnpg-operator
            version: v1.28.1
            patches:
            - resourceRef: pack-cnpg-deployment-patch
              target:
                kind: Deployment
                name: extension-cloudnative-pg
```

Save the shoot spec and wait for the cluster to be reconciled.

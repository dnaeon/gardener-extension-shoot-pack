# gardener-extension-shoot-pack

The `gardener-extension-shoot-pack` repo provides various Kubernetes packages
(packs) for shoot clusters.

# Requirements

- [Go 1.25.x](https://go.dev/) or later
- [GNU Make](https://www.gnu.org/software/make/)
- [Docker](https://www.docker.com/) for local development
- [Gardener Local Setup](https://gardener.cloud/docs/gardener/local_setup/) for local development

# Usage

The table below provides a summary of the available packs, provided by the
extension, which can be installed in shoot clusters.

| Name                  | Version | Description                                                    |
|-----------------------|---------|----------------------------------------------------------------|
| `cnpg-operator`       | v1.20.0 | [CloudNativePG Operator](https://cloudnative-pg.io)            |
| `prometheus-operator` | v0.89.0 | [Prometheus Operator](https://prometheus-operator.dev)         |
| `valkey-operator`     | v0.0.61 | [Valkey Operator](https://docs.hyperspike.io/valkey-operator/) |
| `cert-manager`        | v1.20.0 | [cert-manager](https://cert-manager.io)                        |

You can enable the extension for a [Gardener Shoot cluster](https://gardener.cloud/docs/glossary/#gardener-glossary) by
updating the `.spec.extensions` of your shoot manifest.

The following example enables the extension for a shoot, in which the
CloudNativePG and Prometheus Operators will be installed.

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
          - name: prometheus-operator
            version: v0.89.0
```

Resources provided by [packs can also be customized](./docs/customize-packs.md).

You can also check the [example shoot spec](./examples/shoot.yaml) for a
complete shoot manifest.

# Development

Check the [Development Guide](./docs/development.md) for more details about how
to start developing this extension.

The [Pack Guide](./docs/packs.md) provides information about how to create,
manage, and test packs. It also describes the format of the `PACKAGE` spec.

# Tests

In order to run the tests use the command below:

``` shell
make test
```

In order to test the Helm chart and the manifests provided by it you can run the
following command.

``` shell
make check-helm
```

In order to test the example resources from the `examples/` directory you can
run the following command.

``` shell
make check-examples
```

The `pack-verify` target verifies all packs.

``` shell
make pack-verify
```

# Documentation

Make sure to check the following documents for more information about Gardener
Extensions and the available extensions API.

- [Gardener: Extensibility Overview](https://gardener.cloud/docs/gardener/extensions/)
- [Gardener: Registering Extension Controllers](https://gardener.cloud/docs/gardener/extensions/registration/)
- [Gardener: Extension Resources](https://github.com/gardener/gardener/tree/master/docs/extensions/resources)
- [Gardener: Extensions API Contract](https://github.com/gardener/gardener/blob/master/docs/extensions/resources/extension.md)
- [Gardener: How to Set Up a Gardener Landscape](https://gardener.cloud/docs/gardener/deployment/setup_gardener/)
- [Gardener: Extension API Packages (Go)](https://github.com/gardener/gardener/tree/master/extensions/pkg)

# Contributing

`gardener-extension-shoot-pack` is hosted on
[Github](https://github.com/gardener/gardener-extension-shoot-pack).

Please contribute by reporting issues, suggesting features or by sending patches
using pull requests.

# License

This project is Open Source and licensed under [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0).

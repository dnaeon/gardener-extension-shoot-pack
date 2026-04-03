# gardener-extension-shoot-pack

The `gardener-extension-shoot-pack` repo provides operators pack.

# Requirements

- [Go 1.25.x](https://go.dev/) or later
- [GNU Make](https://www.gnu.org/software/make/)
- [Docker](https://www.docker.com/) for local development
- [Gardener Local Setup](https://gardener.cloud/docs/gardener/local_setup/) for local development

# Code structure

The project repo uses the following code structure.

| Package          | Description                                                                               |
|------------------|-------------------------------------------------------------------------------------------|
| `cmd`            | Command-line application of the extension                                                 |
| `pkg/admission`  | Implementations for the Gardener extension admission `Validator` and `Mutator` interfaces |
| `pkg/apis`       | Extension API types, e.g. configuration spec, etc.                                        |
| `pkg/assets`     | Provides the implementation of pack assets and builtin packs                              |
| `pkg/actuator`   | Implementations for the Gardener Extension `Actuator` interfaces                          |
| `pkg/controller` | Utility wrappers for creating Kubernetes reconcilers for Gardener `Actuators`             |
| `pkg/heartbeat`  | Utility wrappers for creating heartbeat reconcilers for Gardener extensions               |
| `pkg/metrics`    | Metrics emitted by the extension                                                          |
| `pkg/mgr`        | Utility wrappers for creating `controller-runtime` managers using functional options API  |
| `pkg/version`    | Version metadata information about the extension                                          |
| `internal/tools` | Go-based tools used for testing and linting the project                                   |
| `specs`          | Contains the builtin `PACKAGE` specs                                                      |
| `charts`         | Helm charts for deploying the extension                                                   |
| `examples`       | Example Kubernetes resources, which can be used in a dev environment                      |
| `test`           | Various files (e.g. schemas, CRDs, etc.), used during testing                             |

# Usage

You can enable the extension for a [Gardener Shoot cluster](https://gardener.cloud/docs/glossary/#gardener-glossary) by
updating the `.spec.extensions` of your shoot manifest.

``` yaml
...

spec:
  extensions:
    - type: pack
      providerConfig:
        apiVersion: pack.extensions.gardener.cloud/v1alpha1
        kind: PackConfig
        spec:
          foo: bar
```

# Development

Check the [Development Guide](./docs/development.md) for more details about how
to start developing this extension.

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

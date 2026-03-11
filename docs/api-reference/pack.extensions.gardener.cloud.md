# API Reference

## Packages
- [pack.extensions.gardener.cloud/v1alpha1](#packextensionsgardenercloudv1alpha1)


## pack.extensions.gardener.cloud/v1alpha1

Package v1alpha1 provides the v1alpha1 version of the external API types.



#### Pack



Pack describes a pack.



_Appears in:_
- [PackConfigSpec](#packconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name specifies the name of the pack. |  | Required: \{\} <br /> |
| `version` _string_ | Version specifies the version of the pack. |  | Required: \{\} <br /> |




#### PackConfigSpec



PackConfigSpec defines the desired state of [PackConfig]



_Appears in:_
- [PackConfig](#packconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `packs` _[Pack](#pack) array_ | Packs specifies the list of packs to be installed. |  | Required: \{\} <br /> |



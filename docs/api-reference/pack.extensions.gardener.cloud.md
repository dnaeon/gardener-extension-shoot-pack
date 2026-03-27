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
| `patches` _[PatchSpec](#patchspec) array_ | Patches specifies a list of optional patches. |  | Optional: \{\} <br /> |




#### PackConfigSpec



PackConfigSpec defines the desired state of [PackConfig]



_Appears in:_
- [PackConfig](#packconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `packs` _[Pack](#pack) array_ | Packs specifies the list of packs to be installed. |  | Required: \{\} <br /> |


#### PatchSpec



PatchSpec describes a patch for pack resources.



_Appears in:_
- [Pack](#pack)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resourceRef` _string_ | ResourceRef points to a referenced resource name, which is a secret<br />providing patches for pack resources. |  | Required: \{\} <br /> |
| `target` _[Selector](#selector)_ | Target points to the resources that the patch is applied to. |  | Optional: \{\} <br /> |



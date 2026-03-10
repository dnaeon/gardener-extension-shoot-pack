// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package assets

import (
	"embed"
	_ "embed"
	"fmt"
	"io/fs"
	"path/filepath"
)

const (
	// metaFileNamespace specifies the file, which contains the target
	// namespace for pack resources.
	metaFileNamespace = ".NAMESPACE"

	// metaFileDesc specifies the file, which provides a short summary of a
	// pack.
	metaFileDesc = ".DESC"
)

// FS is an [embed.FS], which bundles the builtin packs.
//
//go:embed packs/*/*/*.yaml
var FS embed.FS

// Collection is a set of [Pack] items.
type Collection struct {
	// packs represent the set of packs in the collection.
	Packs []Pack

	// fileSystem is the [fs.FS] from which the collection was created.
	fileSystem fs.FS
}

// New creates a new [Collection] from the given [fs.FS].
// The structure of the filesystem containing the packs follows this
// convention.
//
// All packs resides in the `packs/' top-level directory, where each
// pack stores resources in the `<name>/<version>' sub-directories.
//
// Resources are discovered from the pack base directory, without
// descending into sub-directories.
//
// The following example shows a filesystem with 3 packs in it -
// postgres@17, postgres@18, valkey@9.0.3
//
// packs
// ├── postgres
// │   ├── 17
// │   │   ├── .DESC
// │   │   ├── .NAMESPACE
// │   │   ├── serviceaccount.yaml
// │   │   └── statefulset.yaml
// │   └── 18
// │       ├── .DESC
// │       ├── .NAMESPACE
// │       ├── serviceaccount.yaml
// │       └── statefulset.yaml
// └── valkey
//
//	└── 9.0.3
//	    ├── .DESC
//	    ├── .NAMESPACE
//	    ├── pvc.yaml
//	    └── statefulset.yaml
func New(fileSystem fs.FS) (*Collection, error) {
	topLevelDirs, err := fs.Glob(fileSystem, "packs/*/*")
	if err != nil {
		return nil, err
	}

	packs := make([]Pack, 0)
	for _, packDir := range topLevelDirs {
		// Skip non-directory entries, as these don't represent valid packs.
		item, err := fileSystem.Open(packDir)
		if err != nil {
			return nil, fmt.Errorf("unable to read pack dir %s: %w", packDir, err)
		}
		stat, err := item.Stat()
		if err != nil {
			return nil, fmt.Errorf("unable to stat pack dir %s: %w", packDir, err)
		}

		if !stat.IsDir() {
			continue
		}

		descPath := filepath.Join(packDir, metaFileDesc)
		desc, err := fs.ReadFile(fileSystem, descPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read description file for pack %s: %w", packDir, err)
		}

		namespacePath := filepath.Join(packDir, metaFileNamespace)
		namespace, err := fs.ReadFile(fileSystem, namespacePath)
		if err != nil {
			return nil, fmt.Errorf("unable to read namespace file for pack %s: %w", packDir, err)
		}

		packName := filepath.Base(filepath.Dir(packDir))
		packVersion := filepath.Base(packDir)
		resourcePaths, err := fs.Glob(fileSystem, fmt.Sprintf("%s/*.yaml", packDir))
		if err != nil {
			return nil, fmt.Errorf("unable to list pack resources for pack %s: %w", packDir, err)
		}

		resources := make([]Resource, 0)
		for _, resourcePath := range resourcePaths {
			resource := Resource{
				Path:       resourcePath,
				fileSystem: fileSystem,
			}
			resources = append(resources, resource)
		}

		pack := Pack{
			Name:        packName,
			Version:     packVersion,
			Description: string(desc),
			Namespace:   string(namespace),
			Resources:   resources,
		}

		packs = append(packs, pack)
	}

	c := &Collection{
		Packs:      packs,
		fileSystem: fileSystem,
	}

	return c, nil
}

// Pack reprensets a a collection of Kubernetes resources
type Pack struct {
	// Name specifies the name of the pack.
	Name string

	// Version specifies the pack version.
	Version string

	// Namespace specifies the namespace in which resources will be deployed.
	Namespace string

	// Description provides a short summary of the pack.
	Description string

	// Resources contains the set of resources provided by the pack.
	Resources []Resource
}

// resource represents a resource from a [Pack].
type Resource struct {
	// Path represents the path to the resource.
	Path string

	// fileSystem is the [fs.FS] which contains the resource.
	fileSystem fs.FS
}

// Read reads the resource and returns its contents.
func (r *Resource) Read() ([]byte, error) {
	return fs.ReadFile(r.fileSystem, r.Path)
}

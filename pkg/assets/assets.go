// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package assets

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"embed"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// MetaFile represents a metadata file in a pack.
type MetaFile = string

const (
	// MetaFileNamespace specifies the file, which contains the target
	// namespace for pack resources.
	MetaFileNamespace MetaFile = ".NAMESPACE"

	// MetaFileDesc specifies the file, which provides a short summary of a
	// pack.
	MetaFileDesc MetaFile = ".DESC"

	// MetaFileSums specifies the file, which provides the checksums of pack
	// resources.
	MetaFileSums MetaFile = ".SUMS"
)

// FS is an [embed.FS], which bundles the builtin packs.
//
//go:embed packs/*/*/.DESC
//go:embed packs/*/*/.NAMESPACE
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
// │   │   ├── serviceaccount.yaml
// │   │   └── statefulset.yaml
// │   └── 18
// │       ├── serviceaccount.yaml
// │       └── statefulset.yaml
// └── valkey
//
//	└── 9.0.3
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

		packName := filepath.Base(filepath.Dir(packDir))
		packVersion := filepath.Base(packDir)

		descPath := filepath.Join(packDir, MetaFileDesc)
		desc, err := fs.ReadFile(fileSystem, descPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read description file for pack %s@%s: %w", packName, packVersion, err)
		}

		namespacePath := filepath.Join(packDir, MetaFileNamespace)
		namespace, err := fs.ReadFile(fileSystem, namespacePath)
		if err != nil {
			return nil, fmt.Errorf("unable to read namespace file for pack %s@%s: %w", packName, packVersion, err)
		}

		sumsPath := filepath.Join(packDir, MetaFileSums)
		sumsData, err := fs.ReadFile(fileSystem, sumsPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read sums file for pack %s@%s: %w", packName, packVersion, err)
		}
		sums, err := SumsFromReader(bytes.NewReader(sumsData))
		if err != nil {
			return nil, fmt.Errorf("unable to parse sums file for pack %s@%s: %w", packName, packVersion, err)
		}

		resourcePaths, err := fs.Glob(fileSystem, fmt.Sprintf("%s/*.yaml", packDir))
		if err != nil {
			return nil, fmt.Errorf("unable to list resources for pack %s@%s: %w", packName, packVersion, err)
		}

		resources := make([]Resource, 0)
		for _, resourcePath := range resourcePaths {
			sha256sum, ok := sums[resourcePath]
			if !ok {
				return nil, fmt.Errorf("no checksum found for %s", resourcePath)
			}
			resource := Resource{
				Path:       resourcePath,
				SHA256:     sha256sum,
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

// Resource represents a resource from a [Pack].
type Resource struct {
	// Path represents the path to the resource.
	Path string

	// SHA256 represents the SHA256 checksum of the resource.
	SHA256 string

	// fileSystem is the [fs.FS] which contains the resource.
	fileSystem fs.FS
}

// Read reads the resource and returns its contents.
func (r *Resource) Read() ([]byte, error) {
	return fs.ReadFile(r.fileSystem, r.Path)
}

// Verify verifies the checksum of the resource.
func (r *Resource) Verify() error {
	data, err := r.Read()
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(data)
	sum := fmt.Sprintf("%x", h.Sum(nil))

	if !strings.EqualFold(r.SHA256, sum) {
		return fmt.Errorf("checksum mismatch for %s, want %s, got %s", r.Path, r.SHA256, sum)
	}

	return nil
}

// SumsFromReader parses the checksums from the given reader and returns it as a
// map.
//
// The structure of the sums is expected to follow the conventions used by the
// GNU coreutils tools such as sha256sum(1).
//
// <sha256> /path/to/file/foo
// <sha256> /path/to/file/bar
func SumsFromReader(r io.Reader) (map[string]string, error) {
	sums := make(map[string]string)
	scanner := bufio.NewReader(r)

	for {
		line, err := scanner.ReadString('\n')
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		line = strings.Trim(line, "\n")
		if line == "" {
			continue
		}

		items := strings.Split(line, " ")
		if len(items) != 2 {
			return nil, fmt.Errorf("invalid sums file entry %q", line)
		}
		sums[items[1]] = items[0]
	}

	return sums, nil
}

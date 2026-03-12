// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package assets

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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
//go:embed packs/*/*/.SUMS
//go:embed packs/*/*/*.yaml
var FS embed.FS

// Collection is a set of [Pack] items.
type Collection struct {
	// packs represent the set of packs in the collection.
	Packs []*Pack `json:"packs,omitzero"`

	// fileSystem is the [fs.FS] from which the collection was created.
	fileSystem fs.FS

	// skipVerify will skip the collection verification when set to true.
	skipVerify bool
}

// Verify verifies the checksums of all packs in the [Collection].
func (c *Collection) Verify() error {
	allErrs := make([]error, 0)
	for _, pack := range c.Packs {
		if err := pack.Verify(); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	return utilerrors.NewAggregate(allErrs)
}

// PackExists is predicate which returns true if a [Pack] with the given name
// and version exists in the [Collection], otherwise it returns false.
func (c *Collection) PackExists(name, version string) bool {
	for _, pack := range c.Packs {
		if name == pack.Name && version == pack.Version {
			return true
		}
	}

	return false
}

// GetPack returns the [Pack] with the given name and version, if it exists in
// the [Collection].
func (c *Collection) GetPack(name, version string) (*Pack, error) {
	for _, pack := range c.Packs {
		if name == pack.Name && version == pack.Version {
			return pack, nil
		}
	}

	return nil, fmt.Errorf("unable to find pack %s with version %s", name, version)
}

// Option is a function which configures a pack [Collection].
type Option func(c *Collection)

// WithSkipVerify is an [Option], which skips verification of the packs
// contained within a [Collection].
func WithSkipVerify(val bool) Option {
	opt := func(c *Collection) {
		c.skipVerify = val
	}

	return opt
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
//	packs
//	├── postgres
//	│   ├── 17
//	│   │   ├── serviceaccount.yaml
//	│   │   └── statefulset.yaml
//	│   └── 18
//	│       ├── serviceaccount.yaml
//	│       └── statefulset.yaml
//	└── valkey
//	      └── 9.0.3
//	      ├── pvc.yaml
//	      └── statefulset.yaml
func New(fileSystem fs.FS, opts ...Option) (*Collection, error) {
	topLevelDirs, err := fs.Glob(fileSystem, "packs/*/*")
	if err != nil {
		return nil, err
	}

	packs := make([]*Pack, 0)
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
			sha256sum := sums[resourcePath]
			resource := Resource{
				Path:       resourcePath,
				SHA256:     sha256sum,
				fileSystem: fileSystem,
			}
			resources = append(resources, resource)
		}

		pack := &Pack{
			Name:        packName,
			Version:     packVersion,
			Description: strings.TrimSpace(string(desc)),
			Namespace:   strings.TrimSpace(string(namespace)),
			Resources:   resources,
			BaseDir:     packDir,
			fileSystem:  fileSystem,
		}

		packs = append(packs, pack)
	}

	c := &Collection{
		Packs:      packs,
		fileSystem: fileSystem,
		skipVerify: false,
	}

	for _, opt := range opts {
		opt(c)
	}

	if !c.skipVerify {
		if err := c.Verify(); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Pack reprensets a a collection of Kubernetes resources
type Pack struct {
	// Name specifies the name of the pack.
	Name string `json:"name,omitzero"`

	// Version specifies the pack version.
	Version string `json:"version,omitzero"`

	// Namespace specifies the namespace in which resources will be deployed.
	Namespace string `json:"namespace,omitzero"`

	// Description provides a short summary of the pack.
	Description string `json:"description,omitzero"`

	// Resources contains the set of resources provided by the pack.
	Resources []Resource `json:"resources,omitzero"`

	// BaseDir is the base directory of the pack in the [fs.FS].
	BaseDir string `json:"base_dir,omitzero"`

	// fileSystem is the [fs.FS] which contains the pack.
	fileSystem fs.FS
}

// Verify verifies the checksums of pack resources.
func (p *Pack) Verify() error {
	allErrs := make([]error, 0)

	if p.Name == "" {
		allErrs = append(allErrs, errors.New("missing pack name"))
	}

	if p.Version == "" {
		allErrs = append(allErrs, fmt.Errorf("missing version for pack %s", p.Name))
	}

	if p.Description == "" {
		allErrs = append(allErrs, fmt.Errorf("missing description for pack %s@%s", p.Name, p.Version))
	}

	if p.Namespace == "" {
		allErrs = append(allErrs, fmt.Errorf("missing namespace for pack %s@%s", p.Name, p.Version))
	}

	if len(p.Resources) == 0 {
		allErrs = append(allErrs, fmt.Errorf("no resources in pack %s@%s", p.Name, p.Version))
	}

	for _, resource := range p.Resources {
		if err := resource.Verify(); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	return utilerrors.NewAggregate(allErrs)
}

// ReadFile reads a file from the pack base directory.
func (p *Pack) ReadFile(path string) ([]byte, error) {
	return fs.ReadFile(p.fileSystem, filepath.Join(p.BaseDir, path))
}

// Resource represents a resource from a [Pack].
type Resource struct {
	// Path represents the path to the resource.
	Path string `json:"path,omitzero"`

	// SHA256 represents the SHA256 checksum of the resource.
	SHA256 string `json:"sha256,omitzero"`

	// fileSystem is the [fs.FS] which contains the resource.
	fileSystem fs.FS
}

// Read reads the resource and returns its contents.
func (r *Resource) Read() ([]byte, error) {
	return fs.ReadFile(r.fileSystem, r.Path)
}

// Verify verifies the checksum of the resource.
func (r *Resource) Verify() error {
	if r.SHA256 == "" {
		return fmt.Errorf("missing checksum for %s", r.Path)
	}

	data, err := r.Read()
	if err != nil {
		return err
	}

	h := sha256.New()
	if _, err := h.Write(data); err != nil {
		return err
	}
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
//	<sha256> /path/to/file/foo
//	<sha256> /path/to/file/bar
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

		items := strings.Fields(line)
		if len(items) != 2 {
			return nil, fmt.Errorf("invalid sums file entry %q", line)
		}
		sums[items[1]] = items[0]
	}

	return sums, nil
}

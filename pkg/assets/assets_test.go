// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package assets_test

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-shoot-pack/pkg/assets"
)

// newFS creates a new [fs.FS] from the given path.
func newFS(path string) (fs.FS, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", path)
	}
	filesystem := os.DirFS(path)

	return filesystem, err
}

var _ = Describe("Assets", Ordered, func() {
	It("should successfully create a collection", func() {
		filesystem, err := newFS("testdata/good-collection")
		Expect(err).NotTo(HaveOccurred())
		Expect(filesystem).NotTo(BeNil())

		collection, err := assets.New(filesystem)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(collection).NotTo(BeNil())

		// Validate collection
		Expect(collection.Verify()).To(Succeed())
		Expect(collection.Packs).To(HaveLen(2))
		Expect(collection.PackExists("foo", "v1.0.0")).To(BeTrue())
		Expect(collection.PackExists("bar", "v1.0.0")).To(BeTrue())
		Expect(collection.PackExists("unknown", "v1.0.0")).To(BeFalse())

		// Validate a single pack from the collection
		foo, err := collection.GetPack("foo", "v1.0.0")
		Expect(err).NotTo(HaveOccurred())
		Expect(foo).NotTo(BeNil())

		// Validate pack data
		Expect(foo.Verify()).To(Succeed())
		Expect(foo.Name).To(Equal("foo"))
		Expect(foo.Version).To(Equal("v1.0.0"))
		Expect(foo.BaseDir).To(Equal("packs/foo/v1.0.0"))
		Expect(foo.Description).To(Equal("pack foo"))
		Expect(foo.Resources).To(HaveLen(1))
		Expect(foo.String()).To(Equal("foo@v1.0.0"))

		// Read the checksums metadata file
		sums, err := foo.ReadFile(assets.MetaFileSums)
		Expect(err).NotTo(HaveOccurred())
		Expect(sums).NotTo(BeEmpty())

		// Read a non-existing file
		nilData, err := foo.ReadFile("non-existing-file")
		Expect(err).To(HaveOccurred())
		Expect(nilData).To(BeNil())

		// Get a non-existing pack
		nilPack, err := collection.GetPack("no-such-name", "no-such-version")
		Expect(err).To(MatchError(ContainSubstring("unable to find pack")))
		Expect(nilPack).To(BeNil())
	})

	It("should fail to create collection from a nil filesystem", func() {
		collection, err := assets.New(nil)
		Expect(err).To(HaveOccurred())
		Expect(collection).To(BeNil())
	})

	It("should create an empty pack collection", func() {
		filesystem, err := newFS("testdata/empty-collection")
		Expect(err).NotTo(HaveOccurred())
		Expect(filesystem).NotTo(BeNil())

		collection, err := assets.New(filesystem)
		Expect(err).NotTo(HaveOccurred())
		Expect(collection).NotTo(BeNil())
		Expect(collection.Packs).To(HaveLen(0))
	})

	It("should fail to create collection for pack with missing description metadata", func() {
		filesystem, err := newFS("testdata/missing-metadata-description")
		Expect(err).NotTo(HaveOccurred())
		Expect(filesystem).NotTo(BeNil())

		collection, err := assets.New(filesystem)
		Expect(err).To(MatchError(ContainSubstring("unable to read description file")))
		Expect(collection).To(BeNil())
	})

	It("should fail to create collection for pack with missing sums metadata", func() {
		filesystem, err := newFS("testdata/missing-metadata-sums")
		Expect(err).NotTo(HaveOccurred())
		Expect(filesystem).NotTo(BeNil())

		collection, err := assets.New(filesystem)
		Expect(err).To(MatchError(ContainSubstring("unable to read sums file")))
		Expect(collection).To(BeNil())
	})

	It("should skip pack with no resources", func() {
		filesystem, err := newFS("testdata/missing-pack-resources")
		Expect(err).NotTo(HaveOccurred())
		Expect(filesystem).NotTo(BeNil())

		collection, err := assets.New(filesystem, assets.WithSkipVerify(true))
		Expect(err).NotTo(HaveOccurred())
		Expect(collection).NotTo(BeNil())
		Expect(collection.Packs).To(HaveLen(0))
	})

	It("should skip pack with no version dir", func() {
		filesystem, err := newFS("testdata/missing-pack-version-dir")
		Expect(err).NotTo(HaveOccurred())
		Expect(filesystem).NotTo(BeNil())

		collection, err := assets.New(filesystem)
		Expect(err).NotTo(HaveOccurred())
		Expect(collection).NotTo(BeNil())
		Expect(collection.Packs).To(HaveLen(1))

		// foo pack does not contain a valid basedir
		Expect(collection.PackExists("foo", "no-pack-version-dir")).To(BeFalse())
	})

	It("should fail to create a collection because of invalid checksums", func() {
		filesystem, err := newFS("testdata/invalid-pack-checksums")
		Expect(err).NotTo(HaveOccurred())
		Expect(filesystem).NotTo(BeNil())

		collection, err := assets.New(filesystem)
		Expect(err).To(MatchError(ContainSubstring("checksum mismatch")))
		Expect(collection).To(BeNil())
	})

	It("should skip verification of pack with invalid checksums if requested", func() {
		filesystem, err := newFS("testdata/invalid-pack-checksums")
		Expect(err).NotTo(HaveOccurred())
		Expect(filesystem).NotTo(BeNil())

		collection, err := assets.New(filesystem, assets.WithSkipVerify(true))
		Expect(err).NotTo(HaveOccurred())
		Expect(collection).NotTo(BeNil())
		Expect(collection.Packs).To(HaveLen(1))
	})

	It("should successfully parse a sums file", func() {
		data := `
5fb33765005963b5e17099912e41bbf7b5f5e04ef9b7e84b48bda03c2e2c35c2  packs/foo/v1.0.0/configmap.yaml
c66ce19cb42614398517a8846cd470e169fb6f3cc3f4b6f24598c7a04425299b  packs/bar/v1.0.0/configmap.yaml
`

		sums, err := assets.SumsFromReader(strings.NewReader(data))
		Expect(err).NotTo(HaveOccurred())
		Expect(sums).To(HaveLen(2))

		Expect(sums).To(HaveKeyWithValue("packs/foo/v1.0.0/configmap.yaml", "5fb33765005963b5e17099912e41bbf7b5f5e04ef9b7e84b48bda03c2e2c35c2"))
		Expect(sums).To(HaveKeyWithValue("packs/bar/v1.0.0/configmap.yaml", "c66ce19cb42614398517a8846cd470e169fb6f3cc3f4b6f24598c7a04425299b"))
	})

	It("should fail to parse an invalid sums file", func() {
		data := `
too many fields        packs/foo/v1.0.0/configmap.yaml
another invalid entry  packs/bar/v1.0.0/configmap.yaml
`

		sums, err := assets.SumsFromReader(strings.NewReader(data))
		Expect(err).To(MatchError(ContainSubstring("invalid sums file")))
		Expect(sums).To(BeNil())
	})

	It("should fail to verify invalid pack", func() {
		emptyPack := &assets.Pack{}
		Expect(emptyPack.Verify()).NotTo(Succeed())

		noVersion := &assets.Pack{Name: "foo"}
		Expect(noVersion.Verify()).NotTo(Succeed())

		noDesc := &assets.Pack{Name: "foo", Version: "v1.2.3"}
		Expect(noDesc.Verify()).NotTo(Succeed())

		noResources := &assets.Pack{Name: "foo", Version: "v1.2.3", Description: "foo pack"}
		Expect(noResources.Verify()).NotTo(Succeed())
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package assets_test

import (
	"fmt"
	"io/fs"
	"os"

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
})

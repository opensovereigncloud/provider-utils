// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package osutils_test

import (
	"net"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/ironcore-dev/provider-utils/osutils"
)

var _ = Describe("CleanupSocketIfExists", func() {
	var dir string

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
	})

	It("should succeed if the socket does not exist", func() {
		Expect(CleanupSocketIfExists(filepath.Join(dir, "foo.sock"))).To(Succeed())
	})

	It("should remove an existing socket", func() {
		address := filepath.Join(dir, "foo.sock")
		l, err := net.Listen("unix", address)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = l.Close() }()

		Expect(CleanupSocketIfExists(address)).To(Succeed())
		Expect(address).NotTo(BeAnExistingFile())
	})

	It("should error on an existing non-socket file", func() {
		address := filepath.Join(dir, "foo.sock")
		Expect(os.WriteFile(address, []byte("foo"), 0600)).To(Succeed())

		Expect(CleanupSocketIfExists(address)).To(MatchError(ContainSubstring("is not a socket")))
		Expect(address).To(BeAnExistingFile())
	})
})

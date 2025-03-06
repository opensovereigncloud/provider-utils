// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package host_test

import (
	"os"
	"path/filepath"

	"github.com/ironcore-dev/provider-utils/apiutils/api"
	"github.com/ironcore-dev/provider-utils/storeutils/store"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Store", func() {

	It("should correctly create a object", func(ctx SpecContext) {
		By("creating a watch")
		watch, err := dummyStore.Watch(ctx)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(watch.Stop)

		By("creating a object")
		obj, err := dummyStore.Create(ctx, &Dummy{
			Metadata: api.Metadata{
				ID: "test-id",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(obj).NotTo(BeNil())

		By("checking that the store object exists")
		data, err := os.ReadFile(filepath.Join(tmpDir, obj.ID))
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())

		By("checking that the event got fired")
		event := &store.WatchEvent[*Dummy]{
			Type:   store.WatchEventTypeCreated,
			Object: obj,
		}
		Eventually(watch.Events()).Should(Receive(event))
	})
})

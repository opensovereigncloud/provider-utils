// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package requestutils_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/ironcore-dev/provider-utils/requestutils"
)

var _ = Describe("Cache", func() {
	Describe("NewCache", func() {
		It("should insert and consume a request", func() {
			c := NewCache[string]()

			token, err := c.Insert("foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(token).To(HaveLen(DefaultCacheTokenLen))

			req, found := c.Consume(token)
			Expect(found).To(BeTrue())
			Expect(req).To(Equal("foo"))
		})

		It("should not find an unknown token", func() {
			c := NewCache[string]()

			_, found := c.Consume("non-existent")
			Expect(found).To(BeFalse())
		})

		It("should consume a token only once", func() {
			c := NewCache[string]()

			token, err := c.Insert("foo")
			Expect(err).NotTo(HaveOccurred())

			_, found := c.Consume(token)
			Expect(found).To(BeTrue())

			_, found = c.Consume(token)
			Expect(found).To(BeFalse())
		})

		It("should generate unique tokens for multiple inserted requests", func() {
			c := NewCache[string]()

			seen := map[string]string{}
			for i := 0; i < 100; i++ {
				token, err := c.Insert("req")
				Expect(err).NotTo(HaveOccurred())
				Expect(seen).NotTo(HaveKey(token), "duplicate token generated")
				seen[token] = token
			}
		})

		It("should not consume an expired request", func() {
			c := NewCache[string](func(o *CacheOptions) {
				o.TTL = 20 * time.Millisecond
			})

			token, err := c.Insert("foo")
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(50 * time.Millisecond)

			_, found := c.Consume(token)
			Expect(found).To(BeFalse())
		})

		It("should garbage collect expired requests on insert", func() {
			c := NewCache[string](func(o *CacheOptions) {
				o.TTL = 20 * time.Millisecond
				o.MaxInFlight = 1
			})

			_, err := c.Insert("foo")
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(50 * time.Millisecond)

			By("inserting again, the expired entry should have been garbage collected")
			token, err := c.Insert("bar")
			Expect(err).NotTo(HaveOccurred())

			req, found := c.Consume(token)
			Expect(found).To(BeTrue())
			Expect(req).To(Equal("bar"))
		})

		It("should error with ResourceExhausted when the maximum number of in-flight requests is reached", func() {
			c := NewCache[string](func(o *CacheOptions) {
				o.MaxInFlight = 1
			})

			_, err := c.Insert("foo")
			Expect(err).NotTo(HaveOccurred())

			_, err = c.Insert("bar")
			Expect(err).To(HaveOccurred())
			Expect(status.Code(err)).To(Equal(codes.ResourceExhausted))
		})
	})
})

// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package recorder_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/ironcore-dev/provider-utils/apiutils/api"
	"github.com/ironcore-dev/provider-utils/eventutils/recorder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Machine Event Suite")
}

const (
	maxEvents      = 5
	eventTTL       = 2 * time.Second
	eventType      = "TestType"
	reason         = "TestReason"
	message        = "TestMessage"
	resyncInterval = 2 * time.Second
)

var (
	logOutput   strings.Builder
	log         logr.Logger
	es          *recorder.Store
	apiMetadata = api.Metadata{
		ID: "test-id-1234",
		Annotations: map[string]string{
			"provider-utils.ironcore.dev/annotations": "{\"key1\":\"value1\", \"key2\":\"value2\"}",
			"provider-utils.ironcore.dev/labels": "{" +
				"\"downward-api.machinepoollet.ironcore.dev/root-machine-namespace\":\"default\"," +
				" \"downward-api.machinepoollet.ironcore.dev/root-machine-name\":\"machine1\"}",
		}}
	opts = recorder.EventStoreOptions{
		MaxEvents:      maxEvents,
		TTL:            eventTTL,
		ResyncInterval: resyncInterval,
	}
)

var _ = Describe("Machine EventStore", func() {
	BeforeEach(func() {
		logOutput.Reset()
		log = funcr.New(func(prefix, args string) {
			logOutput.WriteString(args)
		}, funcr.Options{})

		es = recorder.NewEventStore(log, opts)
	})

	Context("Initialization", func() {
		It("should initialize events slice with no elements", func() {
			Expect(es.ListEvents()).To(BeEmpty())
		})
	})

	Context("AddEvent", func() {
		It("should add an event to the store", func() {
			es.Eventf(apiMetadata, eventType, reason, message)
			Expect(logOutput.String()).To(BeEmpty())
			Expect(es.ListEvents()).To(HaveLen(1))
		})

		It("should override the oldest event when the store is full", func() {
			for i := 0; i < maxEvents; i++ {
				es.Eventf(apiMetadata, eventType, reason, "%s %d", message, i)
				Expect(logOutput.String()).To(BeEmpty())
				Expect(es.ListEvents()).To(HaveLen(i + 1))
			}

			es.Eventf(apiMetadata, eventType, reason, "New Event")
			Expect(logOutput.String()).To(BeEmpty())

			events := es.ListEvents()
			Expect(events).To(HaveLen(maxEvents))

			for i := 0; i < maxEvents-1; i++ {
				Expect(events[i].Message).To(Equal(fmt.Sprintf("%s %d", message, i+1)))
			}

			Expect(events[maxEvents-1].Message).To(Equal("New Event"))
		})
	})

	Context("removeExpiredEvents", func() {
		It("should remove events whose TTL has expired", func() {
			es.Eventf(apiMetadata, eventType, reason, message)
			Expect(logOutput.String()).To(BeEmpty())
			Expect(es.ListEvents()).To(HaveLen(1))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go es.Start(ctx)

			Eventually(func(g Gomega) bool {
				return g.Expect(es.ListEvents()).To(HaveLen(0))
			}).WithTimeout(eventTTL + 1*time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
		})

		It("should not remove events whose TTL has not expired", func() {
			es.Eventf(apiMetadata, eventType, reason, message)
			Expect(logOutput.String()).To(BeEmpty())
			Expect(es.ListEvents()).To(HaveLen(1))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go es.Start(ctx)

			Expect(es.ListEvents()).To(HaveLen(1))
		})
	})

	Context("Start", func() {
		It("should periodically remove expired events", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go es.Start(ctx)

			es.Eventf(apiMetadata, eventType, reason, message)
			Expect(logOutput.String()).To(BeEmpty())
			Expect(es.ListEvents()).To(HaveLen(1))

			Eventually(func(g Gomega) bool {
				return g.Expect(es.ListEvents()).To(HaveLen(0))
			}).WithTimeout(resyncInterval + 1*time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
		})
	})

	Context("ListEvents", func() {
		It("should return all current events", func() {
			es.Eventf(apiMetadata, eventType, reason, message)
			Expect(logOutput.String()).To(BeEmpty())

			events := es.ListEvents()
			Expect(events).To(HaveLen(1))
			Expect(events[0].Message).To(Equal(message))
		})

		It("should return a copy of events", func() {
			es.Eventf(apiMetadata, eventType, reason, message)
			Expect(logOutput.String()).To(BeEmpty())
			events := es.ListEvents()
			Expect(events).To(HaveLen(1))

			events[0].Message = "Changed Message"

			storedEvents := es.ListEvents()
			Expect(storedEvents[0].Message).ToNot(Equal(events[0].Message))
		})
	})
})

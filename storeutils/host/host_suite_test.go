// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package host_test

import (
	"testing"
	"time"

	"github.com/ironcore-dev/provider-utils/apiutils/api"
	"github.com/ironcore-dev/provider-utils/storeutils/host"
	"github.com/ironcore-dev/provider-utils/storeutils/store"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type Dummy struct {
	api.Metadata `json:"metadata,omitempty"`
}

var (
	tmpDir     string
	dummyStore store.Store[*Dummy]
)

const (
	eventuallyTimeout    = 5 * time.Second
	pollingInterval      = 250 * time.Millisecond
	consistentlyDuration = 1 * time.Second
)

func TestServer(t *testing.T) {
	SetDefaultConsistentlyPollingInterval(pollingInterval)
	SetDefaultEventuallyPollingInterval(pollingInterval)
	SetDefaultEventuallyTimeout(eventuallyTimeout)
	SetDefaultConsistentlyDuration(consistentlyDuration)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	var err error

	tmpDir = GinkgoT().TempDir()

	dummyStore, err = host.NewStore[*Dummy](host.Options[*Dummy]{
		Dir: tmpDir,
		NewFunc: func() *Dummy {
			return &Dummy{}
		},
	})
	Expect(err).NotTo(HaveOccurred())
})

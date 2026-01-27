// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gpu_test

import (
	"errors"

	"github.com/ironcore-dev/provider-utils/claimutils/claim"
	"github.com/ironcore-dev/provider-utils/claimutils/gpu"
	"github.com/ironcore-dev/provider-utils/claimutils/pci"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type MockReader struct {
	devices []pci.Address
	err     error
}

func (m *MockReader) Read() ([]pci.Address, error) {
	return m.devices, m.err
}

var _ = Describe("GPU Claimer", func() {

	It("should init correct", func(ctx SpecContext) {

		By("init plugin without reader")
		plugin := gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", nil, nil)
		Expect(plugin.Init()).Should(HaveOccurred())

		By("init plugin reader")
		plugin = gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{}, nil)
		Expect(plugin.Init()).ShouldNot(HaveOccurred())

		By("check name")
		Expect(plugin.Name()).Should(Equal("test-plugin"))

		By("init plugin with failing reader")
		testErr := errors.New("test error")
		plugin = gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{
			err: testErr,
		}, nil)
		Expect(plugin.Init()).Should(MatchError(testErr))
	})

	It("should error if no resource left after init", func(ctx SpecContext) {
		By("init plugin")
		plugin := gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{}, nil)
		Expect(plugin.Init()).ShouldNot(HaveOccurred())

		By("claim resources")
		_, err := plugin.Claim(resource.MustParse("1"))
		Expect(err).To(MatchError(claim.ErrInsufficientResources))

	})

	It("should error if no resource left", func(ctx SpecContext) {
		By("init plugin")
		plugin := gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{
			devices: []pci.Address{
				{},
			},
		}, []pci.Address{{}})
		Expect(plugin.Init()).ShouldNot(HaveOccurred())

		By("claim resources")
		_, err := plugin.Claim(resource.MustParse("1"))
		Expect(err).To(MatchError(claim.ErrInsufficientResources))
	})

	It("should claim device if enough are present", func(ctx SpecContext) {
		By("init plugin")
		plugin := gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{
			devices: []pci.Address{
				{},
			},
		}, nil)
		Expect(plugin.Init()).ShouldNot(HaveOccurred())

		By("claim resources")
		gpuClaim, err := plugin.Claim(resource.MustParse("1"))
		Expect(err).NotTo(HaveOccurred())
		Expect(gpuClaim).NotTo(BeNil())

		By("claim resources again and fail")
		secondGpuClaim, err := plugin.Claim(resource.MustParse("1"))
		Expect(err).To(MatchError(claim.ErrInsufficientResources))
		Expect(secondGpuClaim).To(BeNil())
	})

	It("should claim multiple devices", func(ctx SpecContext) {
		By("init plugin")
		plugin := gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{
			devices: []pci.Address{
				{},
				{Function: 1},
			},
		}, nil)
		Expect(plugin.Init()).ShouldNot(HaveOccurred())

		By("claim resources to much resources")
		gpuClaim, err := plugin.Claim(resource.MustParse("10"))
		Expect(err).To(MatchError(claim.ErrInsufficientResources))
		Expect(gpuClaim).To(BeNil())

		By("claim resources")
		_, err = plugin.Claim(resource.MustParse("2"))
		Expect(err).ToNot(HaveOccurred())

		By("claim resources when not sufficient")
		_, err = plugin.Claim(resource.MustParse("1"))
		Expect(err).To(MatchError(claim.ErrInsufficientResources))
	})

	It("should claim different devices", func(ctx SpecContext) {
		By("init plugin")
		plugin := gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{
			devices: []pci.Address{
				{},
				{Function: 1},
			},
		}, nil)
		Expect(plugin.Init()).ShouldNot(HaveOccurred())

		By("claim resources")
		gpuClaim1, err := plugin.Claim(resource.MustParse("1"))
		Expect(err).ToNot(HaveOccurred())

		pciAddress1, ok := gpuClaim1.(gpu.Claim)
		Expect(ok).To(BeTrue())
		Expect(pciAddress1.PCIAddresses()).To(HaveLen(1))

		By("claim resources again")
		gpuClaim2, err := plugin.Claim(resource.MustParse("1"))
		Expect(err).ToNot(HaveOccurred())

		pciAddress2, ok := gpuClaim2.(gpu.Claim)
		Expect(ok).To(BeTrue())
		Expect(pciAddress2.PCIAddresses()).To(HaveLen(1))

		By("ensure claims are not equal")
		Expect(pciAddress1.PCIAddresses()[0]).NotTo(Equal(pciAddress2.PCIAddresses()[0]))
	})

	It("should handle zero-quantity claims", func(ctx SpecContext) {
		By("init plugin")
		plugin := gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{
			devices: []pci.Address{
				{},
				{Function: 1},
			},
		}, nil)
		Expect(plugin.Init()).ShouldNot(HaveOccurred())

		By("claim resources")
		claim, err := plugin.Claim(resource.MustParse("0"))
		Expect(err).ToNot(HaveOccurred())

		gpuClaim, ok := claim.(gpu.Claim)
		Expect(ok).To(BeTrue())
		Expect(gpuClaim.PCIAddresses()).To(BeEmpty())

	})

	It("should release devices", func(ctx SpecContext) {
		By("init plugin")
		plugin := gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{
			devices: []pci.Address{
				{},
				{Function: 1},
			},
		}, []pci.Address{
			{},
			{Function: 1},
		})
		Expect(plugin.Init()).ShouldNot(HaveOccurred())

		gpuClaim := gpu.NewGPUClaim([]pci.Address{
			{},
			{
				Function: 1,
			},
			{
				Function: 2,
			},
		})

		Expect(plugin.Release(gpuClaim)).To(Succeed())

		By("claim resources")
		_, err := plugin.Claim(resource.MustParse("2"))
		Expect(err).ToNot(HaveOccurred())
	})

	It("should fail on generic claim", func(ctx SpecContext) {
		By("init plugin")
		plugin := gpu.NewGPUClaimPlugin(log.FromContext(ctx), "test-plugin", &MockReader{
			devices: []pci.Address{
				{},
				{Function: 1},
			},
		}, []pci.Address{
			{},
			{Function: 1},
		})
		Expect(plugin.Init()).ShouldNot(HaveOccurred())

		By("passing nil claim")
		Expect(plugin.Release(nil)).To(MatchError(claim.ErrInvalidResourceClaim))
	})

})

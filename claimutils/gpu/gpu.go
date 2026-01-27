// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/ironcore-dev/provider-utils/claimutils/claim"
	"github.com/ironcore-dev/provider-utils/claimutils/pci"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Claim interface {
	claim.ResourceClaim
	PCIAddresses() []pci.Address
}

func NewGPUClaim(addresses []pci.Address) Claim {
	return &gpuClaim{
		devices: addresses,
	}
}

type gpuClaim struct {
	devices []pci.Address
}

func (c gpuClaim) PCIAddresses() []pci.Address {
	return c.devices
}

type ClaimStatus bool

const (
	ClaimStatusFree    ClaimStatus = true
	ClaimStatusClaimed ClaimStatus = false
)

func NewGPUClaimPlugin(log logr.Logger, name string, reader pci.Reader, preClaimed []pci.Address) claim.Plugin {

	return &gpuClaimPlugin{
		name:       name,
		log:        log,
		pciReader:  reader,
		devices:    map[pci.Address]ClaimStatus{},
		preClaimed: preClaimed,
	}
}

type gpuClaimPlugin struct {
	name       string
	log        logr.Logger
	devices    map[pci.Address]ClaimStatus
	pciReader  pci.Reader
	preClaimed []pci.Address
}

func (g *gpuClaimPlugin) canClaim(quantity resource.Quantity) bool {
	requested := quantity.Value()

	var free int64
	for _, claimed := range g.devices {
		if claimed == ClaimStatusFree {
			free++
		}
	}
	g.log.V(2).Info("Try to claim devices ", "free", free, "requested", requested)

	return free >= requested
}

func (g *gpuClaimPlugin) CanClaim(quantity resource.Quantity) bool {
	return g.canClaim(quantity)
}

func (g *gpuClaimPlugin) Claim(quantity resource.Quantity) (claim.ResourceClaim, error) {
	if !g.canClaim(quantity) {
		return nil, claim.ErrInsufficientResources
	}

	requested := quantity.Value()

	gClaim := &gpuClaim{}
	for device, claimed := range g.devices {
		if int64(len(gClaim.devices)) == requested {
			break
		}

		if claimed == ClaimStatusFree {
			g.devices[device] = ClaimStatusClaimed
			gClaim.devices = append(gClaim.devices, device)
		}
	}

	g.log.V(2).Info("Claimed devices", "devices", gClaim.devices)

	return gClaim, nil
}

func (g *gpuClaimPlugin) Release(resourceClaim claim.ResourceClaim) error {
	gpu, ok := resourceClaim.(Claim)
	if !ok {
		return claim.ErrInvalidResourceClaim
	}

	pciAddresses := gpu.PCIAddresses()
	for _, pciAddress := range pciAddresses {
		if _, existing := g.devices[pciAddress]; !existing {
			g.log.V(2).Info("Device not managed by this plugin", "pciAddress", pciAddress)
			continue
		}

		g.log.V(3).Info("Unclaimed device", "pciAddress", pciAddress)
		g.devices[pciAddress] = ClaimStatusFree
	}

	return nil
}

func (g *gpuClaimPlugin) Init() error {
	if g.pciReader == nil {
		return errors.New("no reader provided")
	}

	pciDevices, err := g.pciReader.Read()
	if err != nil {
		return fmt.Errorf("failed to read pci devices: %w", err)
	}

	for _, pciDevice := range pciDevices {
		g.log.V(2).Info("Found device", "pciAddress", pciDevice)
		g.devices[pciDevice] = ClaimStatusFree
	}

	for _, pciDevice := range g.preClaimed {
		if _, ok := g.devices[pciDevice]; !ok {
			g.log.V(2).Info("Not discovered pre-claimed pci address", "pciAddress", pciDevice)
			continue
		}

		g.log.V(2).Info("Set device to claimed", "pciAddress", pciDevice)
		g.devices[pciDevice] = ClaimStatusClaimed

	}

	return nil
}

func (g *gpuClaimPlugin) Name() string {
	return g.name
}

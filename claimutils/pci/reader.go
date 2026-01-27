// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pci

import (
	"fmt"
)

type Class uint32
type Vendor uint32

var (
	Class3DController Class = 0x030200

	VendorNvidia Vendor = 0x10de
)

type Address struct {
	Domain   uint
	Bus      uint
	Slot     uint
	Function uint
}

func (p Address) String() string {
	return fmt.Sprintf("%04x:%02x:%02x.%1x", p.Domain, p.Bus, p.Slot, p.Function)
}

type Reader interface {
	Read() ([]Address, error)
}

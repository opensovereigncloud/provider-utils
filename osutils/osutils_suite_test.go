// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package osutils_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOsutils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Osutils Suite")
}

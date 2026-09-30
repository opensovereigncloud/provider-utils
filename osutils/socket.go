// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package osutils

import (
	"errors"
	"fmt"
	"os"
)

// CleanupSocketIfExists deletes any leftover socket at the given address, if any.
//
// If the file at the given address is no socket, an error is returned.
func CleanupSocketIfExists(address string) error {
	stat, err := os.Stat(address)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("error stat-ing socket %q: %w", address, err)
		}
		return nil
	}

	if stat.Mode().Type()&os.ModeSocket == 0 {
		return fmt.Errorf("file at %s is not a socket", address)
	}

	if err := os.Remove(address); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error removing socket: %w", err)
	}
	return nil
}

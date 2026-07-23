//go:build !windows

package winui

import "fmt"

// Run is only supported on Windows.
func Run(cfg Config) error {
	return fmt.Errorf("effexorwinpe-shell GUI requires windows/amd64")
}

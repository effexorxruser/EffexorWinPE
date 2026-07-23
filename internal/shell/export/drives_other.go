//go:build !windows

package export

// OSDriveScanner is unavailable outside Windows.
type OSDriveScanner struct{}

func (OSDriveScanner) List() ([]DriveInfo, error) {
	return nil, nil
}

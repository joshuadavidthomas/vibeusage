//go:build !darwin

package keychain

import "errors"

// ReadGenericPassword is only available on macOS.
func ReadGenericPassword(service, account string) (string, error) {
	_ = service
	_ = account
	return "", errors.New("keychain not available on this platform")
}

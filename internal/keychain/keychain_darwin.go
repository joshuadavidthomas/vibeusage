//go:build darwin

package keychain

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// ReadGenericPassword reads a generic password from macOS Keychain using the
// `security` CLI. If account is empty, the account filter is omitted.
func ReadGenericPassword(service, account string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	args := []string{"find-generic-password", "-s", service}
	if account != "" {
		args = append(args, "-a", account)
	}
	args = append(args, "-w")

	out, err := exec.CommandContext(ctx, "security", args...).Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

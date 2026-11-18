package service

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// DefaultSocketPath prefers XDG_RUNTIME_DIR, which is private to the user.
// The fallback directory in the shared temp dir includes the uid so users
// don't collide, and ensurePrivateDir verifies nobody else owns it.
func DefaultSocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "blueclip", "blueclip.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("blueclip-%d", os.Getuid()), "blueclip.sock")
}

// ensurePrivateDir makes sure dir is a real directory owned by the current
// user and only accessible by them. Another user could otherwise create it
// first in a shared location and swap the socket for their own.
func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create socket directory: %v", err)
	}

	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("failed to stat socket directory: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("socket directory %s is not a directory", dir)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to read owner of socket directory %s", dir)
	}
	if int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("socket directory %s is owned by uid %d, expected %d", dir, stat.Uid, os.Getuid())
	}
	if info.Mode().Perm() != 0700 {
		if err := os.Chmod(dir, 0700); err != nil {
			return fmt.Errorf("failed to set socket directory permissions: %v", err)
		}
	}
	return nil
}

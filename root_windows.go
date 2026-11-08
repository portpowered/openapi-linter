package linter

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func resolveExistingPath(path string) (string, error) {
	if resolved := canonicalDrivePath(path); resolved != "" {
		return resolved, nil
	}
	return filepath.EvalSymlinks(path)
}

// Resolve ordinary local paths through a metadata handle. The caller checks
// the canonical path against its root before opening document contents.
func canonicalDrivePath(path string) string {
	if !ordinaryDrivePath(path) {
		return ""
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	handle, err := windows.CreateFile(name, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return ""
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	var buffer [512]uint16
	length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
	if err != nil || length == 0 || length >= uint32(len(buffer)) {
		return ""
	}
	path, ok := strings.CutPrefix(windows.UTF16ToString(buffer[:length]), "\\\\?\\")
	if !ok || !ordinaryDrivePath(path) {
		return ""
	}
	return path
}

// Leave network, device, relative, and alternate-stream names to filepath.
func ordinaryDrivePath(path string) bool {
	return len(filepath.VolumeName(path)) == 2 && filepath.IsAbs(path) && !strings.Contains(path[2:], ":")
}

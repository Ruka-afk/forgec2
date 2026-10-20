//go:build windows
// +build windows

package main

import (
	"strings"

	"golang.org/x/sys/windows"
)

// fileSearchIsHidden treats dot-prefixed names as hidden and also honours the
// Windows hidden attribute (AppData, NTUSER.DAT, …). A failed attribute query
// is non-fatal: the entry is treated as visible rather than aborting a search.
func fileSearchIsHidden(path, name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return false
	}
	return attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0
}

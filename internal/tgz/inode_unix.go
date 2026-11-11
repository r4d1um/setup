// SPDX-License-Identifier: MIT

package tgz

import (
	"io/fs"
	"syscall"
)

func inode(fi fs.FileInfo) (dev, ino uint64, ok bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink < 2 {
		return 0, 0, false
	}
	return uint64(st.Dev), st.Ino, true
}

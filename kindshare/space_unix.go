//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
)

func checkFreeSpace(path string, offered int64) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return err
	}
	if uint64(stat.Bavail)*uint64(stat.Bsize) < uint64(offered)*2+128*1024*1024 {
		return fmt.Errorf("insufficient free space")
	}
	return nil
}

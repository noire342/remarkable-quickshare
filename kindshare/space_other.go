//go:build !linux

package main

import "os"

func checkFreeSpace(path string, offered int64) error { return os.MkdirAll(path, 0700) }

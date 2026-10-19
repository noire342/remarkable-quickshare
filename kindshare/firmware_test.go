// SPDX-License-Identifier: MIT
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFirmwarePreparationIsExactAndDoesNotMutateOriginal(t *testing.T) {
	original := []byte("header\x00usb0\x00usb1\x00usb1\x00footer")
	snapshot := append([]byte(nil), original...)
	expected := []byte("header\x00usbF\x00usb0\x00usb0\x00footer")
	result, err := prepareImage(original, shaBytes(original), shaBytes(expected))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, expected) || !bytes.Equal(original, snapshot) {
		t.Fatal("unexpected image mutation")
	}
	if _, err := prepareImage(original, "unknown", shaBytes(expected)); err == nil {
		t.Fatal("unknown firmware accepted")
	}
	if _, err := prepareImage(original, shaBytes(original), "wrong result"); err == nil {
		t.Fatal("wrong prepared image accepted")
	}
}

func TestUnsupportedFirmwareLeavesNoOutput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "original")
	out := filepath.Join(root, "prepared")
	os.WriteFile(source, []byte("unknown firmware"), 0600)
	if err := prepareXochitl(source, out); err == nil {
		t.Fatal("unsupported firmware accepted")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("output created for unknown firmware")
	}
	if err := prepareXochitl(source, source); err == nil {
		t.Fatal("overwriting original accepted")
	}
}

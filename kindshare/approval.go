// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Lorenzo Garrone (noire342)
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	defaultSettingsPath     = "/home/root/quickshare/settings.json"
	defaultControlTokenPath = "/home/root/quickshare/control-token"
	defaultControlListen    = "127.0.0.1:12346"
	approvalTimeout         = 60 * time.Second
	uiHeartbeatTimeout      = 40 * time.Second
)

type receiverSettings struct {
	AcceptanceMode string `json:"acceptance_mode"`
}
type offeredDocument struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}
type approvalOffer struct {
	ID        string            `json:"id"`
	Sender    string            `json:"sender"`
	PIN       string            `json:"pin"`
	Files     []offeredDocument `json:"files"`
	TotalSize int64             `json:"total_size"`
	ExpiresAt time.Time         `json:"expires_at"`
}
type pendingApproval struct {
	offer  approvalOffer
	result chan bool
	timer  *time.Timer
}
type approvalState struct {
	Revision uint64           `json:"revision"`
	Settings receiverSettings `json:"settings"`
	UIReady  bool             `json:"ui_ready"`
	Pending  *approvalOffer   `json:"pending"`
}
type approvalManager struct {
	mu               sync.Mutex
	settings         receiverSettings
	path             string
	uiReady          bool
	uiSeen           time.Time
	uiLease          uint64
	heartbeatTimer   *time.Timer
	pending          *pendingApproval
	revision         uint64
	changed          chan struct{}
	now              func() time.Time
	requestTimeout   time.Duration
	heartbeatTimeout time.Duration
}

var activeApproval *approvalManager

func newApprovalManager(path string) *approvalManager {
	m := &approvalManager{path: path, settings: receiverSettings{"ask"}, revision: 1,
		changed: make(chan struct{}), now: func() time.Time { return time.Now().Round(0) }, requestTimeout: approvalTimeout, heartbeatTimeout: uiHeartbeatTimeout}
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("settings unavailable; using confirmation: %v", err)
		}
		return m
	}
	defer f.Close()
	var settings receiverSettings
	decoder := json.NewDecoder(io.LimitReader(f, 4097))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&settings) != nil || decoder.Decode(new(any)) != io.EOF || !validAcceptanceMode(settings.AcceptanceMode) {
		log.Printf("settings invalid; using confirmation")
		return m
	}
	m.settings = settings
	return m
}
func validAcceptanceMode(mode string) bool { return mode == "ask" || mode == "auto" }

func writePrivateFile(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".quickshare-config-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
		err = f.Chmod(0600)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
func loadControlToken(path string) (string, error) {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("control token must be a regular file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		token := strings.TrimSpace(string(data))
		decoded, err := hex.DecodeString(token)
		if err != nil || len(decoded) != 32 || len(token) != 64 {
			return "", fmt.Errorf("invalid control token")
		}
		if err = os.Chmod(path, 0600); err != nil {
			return "", err
		}
		return token, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(secret[:])
	if err := writePrivateFile(path, []byte(token+"\n")); err != nil {
		return "", err
	}
	return token, nil
}
func (m *approvalManager) setMode(mode string) error {
	if !validAcceptanceMode(mode) {
		return fmt.Errorf("acceptance_mode must be ask or auto")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, _ := json.MarshalIndent(receiverSettings{mode}, "", "  ")
	if err := writePrivateFile(m.path, append(data, '\n')); err != nil {
		return err
	}
	if m.settings.AcceptanceMode != mode {
		m.settings.AcceptanceMode = mode
		m.notifyLocked()
	}
	return nil
}
func (m *approvalManager) notifyLocked() {
	m.revision++
	close(m.changed)
	m.changed = make(chan struct{})
}
func (m *approvalManager) readyLocked() bool {
	age := m.now().Sub(m.uiSeen)
	return m.uiReady && age >= 0 && age < m.heartbeatTimeout
}
func (m *approvalManager) finishLocked(accept bool) {
	if m.pending == nil {
		return
	}
	p := m.pending
	m.pending = nil
	if p.timer != nil {
		p.timer.Stop()
	}
	p.result <- accept
	close(p.result)
	m.notifyLocked()
}
func (m *approvalManager) expireLocked() {
	if m.uiReady && !m.readyLocked() {
		m.uiReady = false
		m.notifyLocked()
	}
	if m.pending != nil && (!m.readyLocked() || !m.now().Before(m.pending.offer.ExpiresAt)) {
		m.finishLocked(false)
	}
}
func (m *approvalManager) heartbeat(ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	wasReady := m.readyLocked()
	m.uiSeen = m.now()
	m.uiReady = ready
	m.uiLease++
	lease := m.uiLease
	if m.heartbeatTimer != nil {
		m.heartbeatTimer.Stop()
	}
	if ready {
		m.heartbeatTimer = time.AfterFunc(m.heartbeatTimeout, func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.uiLease == lease {
				m.uiReady = false
				m.notifyLocked()
				m.finishLocked(false)
			}
		})
	}
	if wasReady != ready {
		m.notifyLocked()
	}
	if !ready {
		m.finishLocked(false)
	}
}
func (m *approvalManager) snapshot() (approvalState, <-chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	state := approvalState{Revision: m.revision, Settings: m.settings, UIReady: m.readyLocked()}
	if m.pending != nil {
		copy := m.pending.offer
		copy.Files = append([]offeredDocument(nil), copy.Files...)
		state.Pending = &copy
	}
	return state, m.changed
}

func (m *approvalManager) offer(sender, pin string, files []offeredDocument, total int64) (bool, *pendingApproval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	if m.settings.AcceptanceMode == "auto" {
		return true, nil, nil
	}
	if !m.readyLocked() {
		return false, nil, fmt.Errorf("confirmation UI unavailable")
	}
	if m.pending != nil {
		return false, nil, fmt.Errorf("another confirmation is pending")
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return false, nil, err
	}
	p := &pendingApproval{offer: approvalOffer{ID: hex.EncodeToString(id[:]), Sender: displaySender(sender), PIN: pin, Files: append([]offeredDocument(nil), files...), TotalSize: total, ExpiresAt: m.now().Add(m.requestTimeout)}, result: make(chan bool, 1)}
	m.pending = p
	m.notifyLocked()
	p.timer = time.AfterFunc(m.requestTimeout, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.pending == p {
			m.finishLocked(false)
		}
	})
	return false, p, nil
}
func (m *approvalManager) decide(id string, accept bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	if m.pending == nil || m.pending.offer.ID != id {
		return false
	}
	m.finishLocked(accept)
	return true
}
func (m *approvalManager) cancel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending != nil && m.pending.offer.ID == id {
		m.finishLocked(false)
	}
}
func (m *approvalManager) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.heartbeatTimer != nil {
		m.heartbeatTimer.Stop()
	}
	m.finishLocked(false)
}
func displaySender(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	runes := []rune(strings.TrimSpace(s))
	if len(runes) > 80 {
		runes = runes[:80]
	}
	if len(runes) == 0 {
		return "Unknown device"
	}
	return string(runes)
}

// Quick Share hashes signed Java bytes with a polynomial modulo 9973. This
// independent Go implementation follows Google's Nearby byte_utils algorithm:
// https://github.com/google/nearby/blob/main/internal/platform/byte_utils.cc
func verificationPIN(auth []byte) string {
	hash, multiplier := 0, 1
	for _, b := range auth {
		hash = (hash + int(int8(b))*multiplier) % 9973
		multiplier = multiplier * 31 % 9973
	}
	if hash < 0 {
		hash = -hash
	}
	return fmt.Sprintf("%04d", hash)
}

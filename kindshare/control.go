// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Lorenzo Garrone (noire342)
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func controlHandler(m *approvalManager, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if r.Header.Get("Authorization") == provided || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			controlError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/health":
			state, _ := m.snapshot()
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "ui_ready": state.UIReady, "acceptance_mode": state.Settings.AcceptanceMode, "version": buildVersion})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/state":
			ready, ok := r.URL.Query()["ready"]
			if ok {
				if len(ready) != 1 || (ready[0] != "0" && ready[0] != "1") {
					controlError(w, "ready must be 0 or 1", 400)
					return
				}
				m.heartbeat(ready[0] == "1")
			}
			after, _ := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
			wait, _ := strconv.Atoi(r.URL.Query().Get("wait"))
			if wait < 0 {
				wait = 0
			}
			if wait > 25 {
				wait = 25
			}
			state, changed := m.snapshot()
			if wait > 0 && state.Revision <= after {
				timer := time.NewTimer(time.Duration(wait) * time.Second)
				defer timer.Stop()
				select {
				case <-changed:
				case <-timer.C:
				case <-r.Context().Done():
					return
				}
				state, _ = m.snapshot()
			}
			json.NewEncoder(w).Encode(state)
		case r.Method == http.MethodPut && r.URL.Path == "/v1/settings":
			var settings receiverSettings
			if decodeControlBody(w, r, &settings) != nil || !validAcceptanceMode(settings.AcceptanceMode) {
				controlError(w, "invalid settings", 400)
				return
			}
			if err := m.setMode(settings.AcceptanceMode); err != nil {
				controlError(w, "could not save settings", 500)
				return
			}
			state, _ := m.snapshot()
			json.NewEncoder(w).Encode(state)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/requests/") && strings.HasSuffix(r.URL.Path, "/decision"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/requests/"), "/decision")
			var body struct {
				Decision string `json:"decision"`
			}
			if len(id) != 32 || decodeControlBody(w, r, &body) != nil || (body.Decision != "accept" && body.Decision != "reject") {
				controlError(w, "invalid decision", 400)
				return
			}
			if !m.decide(id, body.Decision == "accept") {
				controlError(w, "request expired or unavailable", http.StatusConflict)
				return
			}
			state, _ := m.snapshot()
			json.NewEncoder(w).Encode(state)
		default:
			controlError(w, "not found", http.StatusNotFound)
		}
	})
}
func controlError(w http.ResponseWriter, message string, status int) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func decodeControlBody(w http.ResponseWriter, r *http.Request, target any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return fmt.Errorf("expected JSON")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
func listenControl(address string, m *approvalManager, token string) (*http.Server, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		return nil, fmt.Errorf("control API must bind exclusively to 127.0.0.1")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	server := &http.Server{Handler: controlHandler(m, token), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 35 * time.Second, MaxHeaderBytes: 4096}
	go server.Serve(listener)
	return server, nil
}
func checkControlAPI(address, token string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		return fmt.Errorf("control health target must be 127.0.0.1")
	}
	req, err := http.NewRequest(http.MethodGet, "http://"+address+"/v1/health", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("control health HTTP %d", response.StatusCode)
	}
	var health struct {
		OK      bool `json:"ok"`
		UIReady bool `json:"ui_ready"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&health); err != nil {
		return err
	}
	if !health.OK || !health.UIReady {
		return fmt.Errorf("confirmation UI is not ready")
	}
	return nil
}
func stopControl(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

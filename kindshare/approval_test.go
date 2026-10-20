// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Lorenzo Garrone (noire342)
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"kindshare/pb/connections"
	"kindshare/pb/sharing"
)

func testManager(t *testing.T) *approvalManager {
	t.Helper()
	m := newApprovalManager(filepath.Join(t.TempDir(), "settings.json"))
	t.Cleanup(m.close)
	return m
}
func TestVerificationPINGoogleVectors(t *testing.T) {
	// Published interoperability vectors, including signed bytes, from
	// https://github.com/google/nearby/blob/main/internal/platform/byte_utils_test.cc
	for _, tt := range []struct {
		input []byte
		want  string
	}{
		{[]byte("rawABCDE"), "0392"}, {[]byte("raw\xd5\x01\xe4\x03\x81"), "6251"}, {nil, "0000"}, {[]byte{255}, "0001"}, {[]byte{128}, "0128"},
	} {
		if got := verificationPIN(tt.input); got != tt.want {
			t.Errorf("%x: %s != %s", tt.input, got, tt.want)
		}
	}
}
func TestSettingsDefaultAndPersistence(t *testing.T) {
	for _, input := range []string{"", `{}`, `null`, `{"acceptance_mode":"unknown"}`, `{"acceptance_mode":"auto"} trailing`, `{"acceptance_mode":"auto","unexpected":true}`} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if input != "" {
				os.WriteFile(path, []byte(input), 0600)
			}
			m := newApprovalManager(path)
			defer m.close()
			state, _ := m.snapshot()
			if state.Settings.AcceptanceMode != "ask" {
				t.Fatal("unsafe default")
			}
		})
	}
	m := testManager(t)
	if err := m.setMode("auto"); err != nil {
		t.Fatal(err)
	}
	reloaded := newApprovalManager(m.path)
	defer reloaded.close()
	state, _ := reloaded.snapshot()
	if state.Settings.AcceptanceMode != "auto" {
		t.Fatal("mode not persisted")
	}
	if err := m.setMode("invalid"); err == nil {
		t.Fatal("invalid mode persisted")
	}
	if err := m.setMode("ask"); err != nil {
		t.Fatal(err)
	}
	reloaded = newApprovalManager(m.path)
	defer reloaded.close()
	state, _ = reloaded.snapshot()
	if state.Settings.AcceptanceMode != "ask" {
		t.Fatal("mode not updated")
	}
}
func TestControlTokenPersistsAndRejectsInvalidInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	first, err := loadControlToken(path)
	if err != nil || len(first) != 64 {
		t.Fatal(first, err)
	}
	second, err := loadControlToken(path)
	if err != nil || first != second {
		t.Fatal("token changed", err)
	}
	os.WriteFile(path, []byte("not-a-token"), 0600)
	if _, err := loadControlToken(path); err == nil {
		t.Fatal("invalid token accepted")
	}
}
func TestApprovalSnapshotsModeAndAllowsOnlyOnePending(t *testing.T) {
	m := testManager(t)
	files := []offeredDocument{{"book.pdf", 50}}
	if _, _, err := m.offer("phone", "1234", files, 50); err == nil {
		t.Fatal("offer accepted without UI")
	}
	m.heartbeat(true)
	auto, pending, err := m.offer("phone\u202e\n", "1234", files, 50)
	if err != nil || auto {
		t.Fatal(auto, err)
	}
	if pending.offer.Sender != "phone" {
		t.Fatal("sender display control characters retained")
	}
	if _, _, err := m.offer("other", "4321", files, 50); err == nil {
		t.Fatal("concurrent prompt accepted")
	}
	if err := m.setMode("auto"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pending.result:
		t.Fatal("changing mode resolved existing prompt")
	default:
	}
	auto, next, err := m.offer("next", "4321", files, 50)
	if err != nil || !auto || next != nil {
		t.Fatal("new offer did not use auto mode")
	}
	if !m.decide(pending.offer.ID, true) {
		t.Fatal("pending manual prompt lost")
	}
	if accepted := <-pending.result; !accepted {
		t.Fatal("manual confirmation rejected")
	}
	if m.decide(pending.offer.ID, true) {
		t.Fatal("stale decision replayed")
	}
}
func TestApprovalExpiryAndLostUI(t *testing.T) {
	for _, reason := range []string{"timeout", "heartbeat", "locked", "reconnect-after-expiry", "clock-moved-backward"} {
		t.Run(reason, func(t *testing.T) {
			m := testManager(t)
			now := time.Now().Round(0)
			m.now = func() time.Time { return now }
			m.heartbeat(true)
			_, p, err := m.offer("phone", "0001", []offeredDocument{{"book.pdf", 1}}, 1)
			if err != nil {
				t.Fatal(err)
			}
			switch reason {
			case "timeout":
				now = now.Add(61 * time.Second)
			case "heartbeat":
				now = now.Add(41 * time.Second)
			case "locked":
				m.heartbeat(false)
			case "reconnect-after-expiry":
				now = now.Add(61 * time.Second)
				m.heartbeat(true)
			case "clock-moved-backward":
				now = now.Add(-time.Minute)
			}
			if m.decide(p.offer.ID, true) {
				t.Fatal("expired/unavailable UI decision accepted")
			}
			if accepted := <-p.result; accepted {
				t.Fatal("unattended offer accepted")
			}
			state, _ := m.snapshot()
			if state.Pending != nil {
				t.Fatal("stale prompt retained")
			}
		})
	}
	m := testManager(t)
	if err := m.setMode("auto"); err != nil {
		t.Fatal(err)
	}
	if automatic, p, err := m.offer("phone", "0001", nil, 0); err != nil || !automatic || p != nil {
		t.Fatal("explicit auto mode depends on UI")
	}
}

func awaitPendingOffer(t *testing.T, m *approvalManager) *approvalOffer {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		state, changed := m.snapshot()
		if state.Pending != nil {
			return state.Pending
		}
		select {
		case <-changed:
		case <-timer.C:
			t.Fatal("no pending offer")
			return nil
		}
	}
}
func TestEncryptedReadLoopAcceptsAndTransfersOnlyAfterDecision(t *testing.T) {
	m := testManager(t)
	m.heartbeat(true)
	spool := filepath.Join(t.TempDir(), "spool")
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	client.SetDeadline(time.Now().Add(3 * time.Second))
	done := make(chan struct{})
	go func() { defer close(done); runSharingSession(server, testSessionKeys(), "phone", spool, m) }()
	peer := newSession(client, testSessionKeys(), "")
	if err := peer.sendSharingFrame(testIntroduction()); err != nil {
		t.Fatal(err)
	}
	pending := awaitPendingOffer(t, m)
	if _, err := os.Stat(spool); !os.IsNotExist(err) {
		t.Fatal("read loop wrote before approval")
	}
	if !m.decide(pending.ID, true) {
		t.Fatal("confirmation was rejected")
	}
	response := smokeSharingResponse(t, peer)
	if response.GetV1().GetConnectionResponse().GetStatus() != sharing.ConnectionResponseFrame_ACCEPT {
		t.Fatal("missing encrypted acceptance")
	}
	content := []byte("%PDF-1.4\n")
	packet := &connections.OfflineFrame{Version: connections.OfflineFrame_V1.Enum(), V1: &connections.V1Frame{Type: connections.V1Frame_PAYLOAD_TRANSFER.Enum(), PayloadTransfer: &connections.PayloadTransferFrame{PacketType: connections.PayloadTransferFrame_DATA.Enum(), PayloadHeader: &connections.PayloadTransferFrame_PayloadHeader{Id: proto.Int64(42), Type: connections.PayloadTransferFrame_PayloadHeader_FILE.Enum(), TotalSize: proto.Int64(int64(len(content)))}, PayloadChunk: &connections.PayloadTransferFrame_PayloadChunk{Offset: proto.Int64(0), Body: content, Flags: proto.Int32(1)}}}}
	if err := peer.sendOffline(packet); err != nil {
		t.Fatal(err)
	}
	for _, expectedType := range []connections.V1Frame_FrameType{connections.V1Frame_PAYLOAD_TRANSFER, connections.V1Frame_DISCONNECTION} {
		raw, err := readFrame(client)
		if err != nil {
			t.Fatal(err)
		}
		clear, _, err := peer.keys.decrypt(raw)
		if err != nil {
			t.Fatal(err)
		}
		var frame connections.OfflineFrame
		proto.Unmarshal(clear, &frame)
		if frame.GetV1().GetType() != expectedType {
			t.Fatal("unexpected response", frame.String())
		}
		if expectedType == connections.V1Frame_PAYLOAD_TRANSFER && frame.GetV1().GetPayloadTransfer().GetPacketType() != connections.PayloadTransferFrame_PAYLOAD_ACK {
			t.Fatal("file not acknowledged")
		}
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("completed session did not terminate")
	}
	entries, err := os.ReadDir(spool)
	if err != nil || len(entries) != 1 {
		t.Fatal("wrong transfer count", err)
	}
	data, err := os.ReadFile(filepath.Join(spool, entries[0].Name(), "book.pdf"))
	if err != nil || !bytes.Equal(data, content) {
		t.Fatal("approved content not queued intact", err)
	}
}

func controlRequest(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}
func TestControlAPIRequiresAuthenticationAndHealthDoesNotMarkUIReady(t *testing.T) {
	m := testManager(t)
	token := strings.Repeat("a", 64)
	handler := controlHandler(m, token)
	if got := controlRequest(handler, "GET", "/v1/state?ready=1", "", "wrong").Code; got != 401 {
		t.Fatal(got)
	}
	if got := controlRequest(handler, "GET", "/v1/health", "", token).Code; got != 200 {
		t.Fatal(got)
	}
	if got := controlRequest(handler, "GET", "/v1/state", "", token).Code; got != 200 {
		t.Fatal(got)
	}
	state, _ := m.snapshot()
	if state.UIReady {
		t.Fatal("read-only health/state pretended to be UI")
	}
	if got := controlRequest(handler, "GET", "/v1/state?ready=1", "", token).Code; got != 200 {
		t.Fatal(got)
	}
	state, _ = m.snapshot()
	if !state.UIReady {
		t.Fatal("UI heartbeat ignored")
	}
	if got := controlRequest(handler, "PUT", "/v1/settings", `{"acceptance_mode":"auto"}`, token).Code; got != 200 {
		t.Fatal(got)
	}
	if got := controlRequest(handler, "PUT", "/v1/settings", `{"acceptance_mode":"invalid"}`, token).Code; got != 400 {
		t.Fatal(got)
	}
	if got := controlRequest(handler, "PUT", "/v1/settings", `{"acceptance_mode":"ask","extra":true}`, token).Code; got != 400 {
		t.Fatal(got)
	}
	if _, err := listenControl("0.0.0.0:0", m, token); err == nil {
		t.Fatal("control API exposed outside loopback")
	}
}
func TestControlDecisionCannotBeReplayedOrApproveAnotherRequest(t *testing.T) {
	m := testManager(t)
	m.heartbeat(true)
	_, p, err := m.offer("phone", "1234", []offeredDocument{{"book.pdf", 1}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	handler := controlHandler(m, "secret")
	if got := controlRequest(handler, "POST", "/v1/requests/"+strings.Repeat("f", 32)+"/decision", `{"decision":"accept"}`, "secret").Code; got != 409 {
		t.Fatal(got)
	}
	path := "/v1/requests/" + p.offer.ID + "/decision"
	if got := controlRequest(handler, "POST", path, `{"decision":"reject"}`, "secret").Code; got != 200 {
		t.Fatal(got)
	}
	if accepted := <-p.result; accepted {
		t.Fatal("reject became accept")
	}
	if got := controlRequest(handler, "POST", path, `{"decision":"accept"}`, "secret").Code; got != 409 {
		t.Fatal(got)
	}
}
func TestControlLongPollObservesSettingsChanges(t *testing.T) {
	m := testManager(t)
	m.heartbeat(true)
	state, _ := m.snapshot()
	handler := controlHandler(m, "secret")
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		result <- controlRequest(handler, "GET", fmt.Sprintf("/v1/state?after=%d&wait=25&ready=1", state.Revision), "", "secret")
	}()
	if err := m.setMode("auto"); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-result:
		var received approvalState
		if json.Unmarshal(response.Body.Bytes(), &received) != nil || received.Settings.AcceptanceMode != "auto" {
			t.Fatal(response.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("long poll did not wake")
	}
}

type bufferedTestConn struct{ bytes.Buffer }

func (*bufferedTestConn) Close() error                     { return nil }
func (*bufferedTestConn) LocalAddr() net.Addr              { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }
func (*bufferedTestConn) RemoteAddr() net.Addr             { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }
func (*bufferedTestConn) SetDeadline(time.Time) error      { return nil }
func (*bufferedTestConn) SetReadDeadline(time.Time) error  { return nil }
func (*bufferedTestConn) SetWriteDeadline(time.Time) error { return nil }
func testSessionKeys() *sessionKeys {
	return &sessionKeys{encryptKey: bytes.Repeat([]byte{1}, 32), decryptKey: bytes.Repeat([]byte{1}, 32), sendHMAC: bytes.Repeat([]byte{2}, 32), receiveHMAC: bytes.Repeat([]byte{2}, 32), authString: bytes.Repeat([]byte{3}, 32)}
}
func testIntroduction() *sharing.Frame {
	return &sharing.Frame{Version: sharing.Frame_V1.Enum(), V1: &sharing.V1Frame{Type: sharing.V1Frame_INTRODUCTION.Enum(), Introduction: &sharing.IntroductionFrame{FileMetadata: []*sharing.FileMetadata{{Name: proto.String("book.pdf"), PayloadId: proto.Int64(42), Size: proto.Int64(9), Type: sharing.FileMetadata_DOCUMENT.Enum()}}}}}
}
func TestSessionDoesNotCreateSpoolBeforeConfirmationAndRejectsEarlyContent(t *testing.T) {
	m := testManager(t)
	m.heartbeat(true)
	path := filepath.Join(t.TempDir(), "absent")
	s := newSession(&bufferedTestConn{}, testSessionKeys(), path)
	s.controller = m
	s.sender = "phone"
	defer s.cleanup()
	if err := s.handleSharing(testIntroduction()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("spool created before approval")
	}
	packet := &connections.PayloadTransferFrame{PayloadHeader: &connections.PayloadTransferFrame_PayloadHeader{Id: proto.Int64(42), Type: connections.PayloadTransferFrame_PayloadHeader_FILE.Enum()}, PayloadChunk: &connections.PayloadTransferFrame_PayloadChunk{Body: []byte("%PDF-1.4\n")}}
	if s.handlePayload(packet) == nil {
		t.Fatal("early content did not close session")
	}
	s.cleanup()
	state, _ := m.snapshot()
	if state.Pending != nil {
		t.Fatal("disconnect did not withdraw prompt")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("early content created spool")
	}
}
func TestSessionApprovalAndAutomaticModes(t *testing.T) {
	for _, mode := range []string{"ask", "auto", "reject"} {
		t.Run(mode, func(t *testing.T) {
			m := testManager(t)
			if mode == "auto" {
				m.setMode("auto")
			} else {
				m.heartbeat(true)
			}
			path := filepath.Join(t.TempDir(), "spool")
			s := newSession(&bufferedTestConn{}, testSessionKeys(), path)
			s.controller = m
			defer s.cleanup()
			if err := s.handleSharing(testIntroduction()); err != nil {
				t.Fatal(err)
			}
			if mode != "auto" {
				state, _ := m.snapshot()
				if state.Pending == nil {
					t.Fatal("missing pending offer")
				}
				m.decide(state.Pending.ID, mode != "reject")
				accepted := <-s.approvalResult
				err := s.finishApproval(accepted)
				if mode == "reject" {
					if err == nil {
						t.Fatal("reject left session open")
					}
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("reject wrote spool")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(s.files) != 1 {
				t.Fatal("approved file not armed")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestConcurrentEncryptedWritesHaveContiguousSequenceAndCompleteFrames(t *testing.T) {
	conn := &bufferedTestConn{}
	keys := testSessionKeys()
	s := newSession(conn, keys, "")
	var wg sync.WaitGroup
	errors := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- s.sendKeepAliveAck() }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 50; i++ {
		raw, err := readFrame(conn)
		if err != nil {
			t.Fatal(err)
		}
		_, seq, err := keys.decrypt(raw)
		if err != nil || seq != int32(i) {
			t.Fatal("interleaved/reordered encrypted frame", seq, err)
		}
	}
	if _, err := readFrame(conn); err != io.EOF {
		t.Fatal("extra/corrupt bytes", err)
	}
}

func TestEncryptedReadLoopHandlesKeepAliveAndDisconnectWhileApprovalPending(t *testing.T) {
	m := testManager(t)
	m.heartbeat(true)
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	client.SetDeadline(time.Now().Add(3 * time.Second))
	done := make(chan struct{})
	go func() {
		defer close(done)
		runSharingSession(server, testSessionKeys(), "phone", filepath.Join(t.TempDir(), "spool"), m)
	}()
	peer := newSession(client, testSessionKeys(), "")
	if err := peer.sendSharingFrame(testIntroduction()); err != nil {
		t.Fatal(err)
	}
	if err := peer.sendOffline(&connections.OfflineFrame{Version: connections.OfflineFrame_V1.Enum(), V1: &connections.V1Frame{Type: connections.V1Frame_KEEP_ALIVE.Enum(), KeepAlive: &connections.KeepAliveFrame{}}}); err != nil {
		t.Fatal(err)
	}
	raw, err := readFrame(client)
	if err != nil {
		t.Fatal(err)
	}
	clear, _, err := peer.keys.decrypt(raw)
	if err != nil {
		t.Fatal(err)
	}
	var frame connections.OfflineFrame
	proto.Unmarshal(clear, &frame)
	if frame.GetV1().GetType() != connections.V1Frame_KEEP_ALIVE || !frame.GetV1().GetKeepAlive().GetAck() {
		t.Fatal("keepalive stalled behind approval")
	}
	state, _ := m.snapshot()
	if state.Pending == nil {
		t.Fatal("offer no longer pending")
	}
	client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reader did not handle disconnect")
	}
	state, _ = m.snapshot()
	if state.Pending != nil {
		t.Fatal("orphaned pending request")
	}
}

func TestAuthenticatedSharingCancelWithdrawsPendingOfferImmediately(t *testing.T) {
	m := testManager(t)
	m.heartbeat(true)
	spool := filepath.Join(t.TempDir(), "spool")
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	client.SetDeadline(time.Now().Add(3 * time.Second))
	done := make(chan struct{})
	go func() { defer close(done); runSharingSession(server, testSessionKeys(), "phone", spool, m) }()
	peer := newSession(client, testSessionKeys(), "")
	if err := peer.sendSharingFrame(testIntroduction()); err != nil {
		t.Fatal(err)
	}
	pending := awaitPendingOffer(t, m)
	if err := peer.sendSharingFrame(&sharing.Frame{Version: sharing.Frame_V1.Enum(), V1: &sharing.V1Frame{Type: sharing.V1Frame_CANCEL.Enum()}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("authenticated CANCEL did not terminate session")
	}
	state, _ := m.snapshot()
	if state.Pending != nil {
		t.Fatal("cancelled request remains visible")
	}
	if m.decide(pending.ID, true) {
		t.Fatal("cancelled request can still be approved")
	}
	if _, err := os.Stat(spool); !os.IsNotExist(err) {
		t.Fatal("cancelled request wrote spool")
	}
}

func TestPayloadControlCancellationWithdrawsOnlyRelevantOffer(t *testing.T) {
	for _, event := range []connections.PayloadTransferFrame_ControlMessage_EventType{connections.PayloadTransferFrame_ControlMessage_PAYLOAD_CANCELED, connections.PayloadTransferFrame_ControlMessage_PAYLOAD_ERROR} {
		for _, payload := range []string{"file", "introduction"} {
			t.Run(event.String()+"/"+payload, func(t *testing.T) {
				m := testManager(t)
				m.heartbeat(true)
				spool := filepath.Join(t.TempDir(), "spool")
				s := newSession(&bufferedTestConn{}, testSessionKeys(), spool)
				s.controller = m
				defer s.cleanup()
				if err := s.handleSharing(testIntroduction()); err != nil {
					t.Fatal(err)
				}
				state, _ := m.snapshot()
				pending := state.Pending
				if pending == nil {
					t.Fatal("missing prompt")
				}
				// An unrelated canceled control payload cannot dismiss the prompt.
				packet := &connections.PayloadTransferFrame{PacketType: connections.PayloadTransferFrame_CONTROL.Enum(), PayloadHeader: &connections.PayloadTransferFrame_PayloadHeader{Id: proto.Int64(900)}, ControlMessage: &connections.PayloadTransferFrame_ControlMessage{Event: event.Enum()}}
				if err := s.handlePayload(packet); err != nil {
					t.Fatal("unrelated cancellation closed session", err)
				}
				state, _ = m.snapshot()
				if state.Pending == nil {
					t.Fatal("unrelated cancellation dismissed prompt")
				}
				id := int64(42)
				if payload == "introduction" {
					id = 500
					s.introductionPayloadID = id
					s.hasIntroductionPayloadID = true
				}
				packet.PayloadHeader.Id = proto.Int64(id)
				if err := s.handlePayload(packet); err == nil {
					t.Fatal("payload cancellation did not terminate session")
				}
				state, _ = m.snapshot()
				if state.Pending != nil {
					t.Fatal("payload cancellation retained prompt")
				}
				if m.decide(pending.ID, true) {
					t.Fatal("payload cancellation can be overridden")
				}
				if _, err := os.Stat(spool); !os.IsNotExist(err) {
					t.Fatal("CONTROL packet created spool")
				}
			})
		}
	}
}

func TestPayloadAckDoesNotOpenOfferedFile(t *testing.T) {
	m := testManager(t)
	m.setMode("auto")
	spool := filepath.Join(t.TempDir(), "spool")
	s := newSession(&bufferedTestConn{}, testSessionKeys(), spool)
	s.controller = m
	defer s.cleanup()
	if err := s.handleSharing(testIntroduction()); err != nil {
		t.Fatal(err)
	}
	packet := &connections.PayloadTransferFrame{PacketType: connections.PayloadTransferFrame_PAYLOAD_ACK.Enum(), PayloadHeader: &connections.PayloadTransferFrame_PayloadHeader{Id: proto.Int64(42), Type: connections.PayloadTransferFrame_PayloadHeader_FILE.Enum()}}
	if err := s.handlePayload(packet); err != nil {
		t.Fatal(err)
	}
	if s.files[42].f != nil {
		t.Fatal("ACK opened a file")
	}
	if _, err := os.Stat(s.files[42].partial); !os.IsNotExist(err) {
		t.Fatal("ACK wrote partial file")
	}
}

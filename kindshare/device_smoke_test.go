package main

// Opt-in live protocol test. Uses an independent client role, not an Android sender.
import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
	"google.golang.org/protobuf/proto"
	"kindshare/pb/connections"
	"kindshare/pb/securegcm"
	"kindshare/pb/securemessage"
	"kindshare/pb/sharing"
)

func marshalSmoke(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func smokeClientKeys(t *testing.T, c net.Conn) *sessionKeys {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	point := priv.PublicKey().Bytes()
	pub := marshalSmoke(t, &securemessage.GenericPublicKey{Type: securemessage.PublicKeyType_EC_P256.Enum(), EcP256PublicKey: &securemessage.EcP256PublicKey{X: javaBigInt(new(big.Int).SetBytes(point[1:33])), Y: javaBigInt(new(big.Int).SetBytes(point[33:]))}})
	finish := marshalSmoke(t, &securegcm.Ukey2Message{MessageType: securegcm.Ukey2Message_CLIENT_FINISH.Enum(), MessageData: marshalSmoke(t, &securegcm.Ukey2ClientFinished{PublicKey: pub})})
	sum := sha512.Sum512(finish)
	init := marshalSmoke(t, &securegcm.Ukey2Message{MessageType: securegcm.Ukey2Message_CLIENT_INIT.Enum(), MessageData: marshalSmoke(t, &securegcm.Ukey2ClientInit{Version: proto.Int32(1), Random: randBytes(32), NextProtocol: proto.String(nextProtocol), CipherCommitments: []*securegcm.Ukey2ClientInit_CipherCommitment{{HandshakeCipher: securegcm.Ukey2HandshakeCipher_P256_SHA512.Enum(), Commitment: sum[:]}}})})
	if err := writeFrame(c, init); err != nil {
		t.Fatal(err)
	}
	serverRaw, err := readFrame(c)
	if err != nil {
		t.Fatal(err)
	}
	var msg securegcm.Ukey2Message
	proto.Unmarshal(serverRaw, &msg)
	var server securegcm.Ukey2ServerInit
	proto.Unmarshal(msg.GetMessageData(), &server)
	var serverPub securemessage.GenericPublicKey
	proto.Unmarshal(server.GetPublicKey(), &serverPub)
	ec := serverPub.GetEcP256PublicKey()
	if ec == nil {
		t.Fatal("missing server key")
	}
	serverPoint := append([]byte{4}, append(pad32(ec.GetX()), pad32(ec.GetY())...)...)
	peer, err := ecdh.P256().NewPublicKey(serverPoint)
	if err != nil {
		t.Fatal(err)
	}
	dh, err := priv.ECDH(peer)
	if err != nil {
		t.Fatal(err)
	}
	ikm := sha256.Sum256(dh)
	info := append(append([]byte{}, init...), serverRaw...)
	next := hkdfBytes(ikm[:], []byte("UKEY2 v1 next"), info, 32)
	authString := hkdfBytes(ikm[:], []byte("UKEY2 v1 auth"), info, 32)
	clientKey := hkdfBytes(next, saltD2D, []byte("client"), 32)
	serverKey := hkdfBytes(next, saltD2D, []byte("server"), 32)
	if err := writeFrame(c, finish); err != nil {
		t.Fatal(err)
	}
	return &sessionKeys{authString: authString, encryptKey: hkdfBytes(clientKey, saltEncMac, []byte("ENC:2"), 32), sendHMAC: hkdfBytes(clientKey, saltEncMac, []byte("SIG:1"), 32), decryptKey: hkdfBytes(serverKey, saltEncMac, []byte("ENC:2"), 32), receiveHMAC: hkdfBytes(serverKey, saltEncMac, []byte("SIG:1"), 32)}
}

func smokeSharingResponse(t *testing.T, s *session) *sharing.Frame {
	t.Helper()
	buffers := map[int64][]byte{}
	for {
		raw, err := readFrame(s.conn)
		if err != nil {
			t.Fatal(err)
		}
		clear, _, err := s.keys.decrypt(raw)
		if err != nil {
			t.Fatal(err)
		}
		var off connections.OfflineFrame
		if err := proto.Unmarshal(clear, &off); err != nil {
			t.Fatal(err)
		}
		pt := off.GetV1().GetPayloadTransfer()
		if pt == nil {
			continue
		}
		id := pt.GetPayloadHeader().GetId()
		chunk := pt.GetPayloadChunk()
		buffers[id] = append(buffers[id], chunk.GetBody()...)
		if chunk.GetFlags()&1 != 0 {
			var result sharing.Frame
			if err := proto.Unmarshal(buffers[id], &result); err != nil {
				t.Fatal(err)
			}
			return &result
		}
	}
}

func TestDeviceDiscovery(t *testing.T) {
	host := os.Getenv("REMARKABLE_TEST_HOST")
	if host == "" {
		t.Skip("opt-in device test")
	}
	q := new(dns.Msg)
	q.SetQuestion(serviceType+".local.", dns.TypePTR)
	q.Id = 0
	q.Question[0].Qclass = dns.ClassINET | unicastBit
	r, _, err := (&dns.Client{Timeout: 5 * time.Second}).Exchange(q, net.JoinHostPort(host, "5353"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Answer) == 0 {
		t.Fatal("no discovery answer")
	}
	t.Log(r.String())
}

// This opt-in probe shows a real confirmation without sending document data.
// It cancels after 20 seconds; do not press Accept during this particular check.
func TestDevicePromptCancellation(t *testing.T) {
	host := os.Getenv("REMARKABLE_TEST_HOST")
	if host == "" || os.Getenv("REMARKABLE_TEST_PROMPT_PROBE") != "1" {
		t.Skip("opt-in physical prompt probe")
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, "12345"), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(35 * time.Second))
	request := &connections.OfflineFrame{Version: connections.OfflineFrame_V1.Enum(), V1: &connections.V1Frame{Type: connections.V1Frame_CONNECTION_REQUEST.Enum(), ConnectionRequest: &connections.ConnectionRequestFrame{EndpointId: proto.String("TEST"), EndpointName: proto.String("PC verifica popup")}}}
	if err := writeFrame(c, marshalSmoke(t, request)); err != nil {
		t.Fatal(err)
	}
	keys := smokeClientKeys(t, c)
	if err := sendConnectionResponse(c); err != nil {
		t.Fatal(err)
	}
	if _, err := readFrame(c); err != nil {
		t.Fatal(err)
	}
	s := newSession(c, keys, "")
	for _, frameType := range []sharing.V1Frame_FrameType{sharing.V1Frame_PAIRED_KEY_ENCRYPTION, sharing.V1Frame_PAIRED_KEY_RESULT} {
		frame := &sharing.Frame{Version: sharing.Frame_V1.Enum(), V1: &sharing.V1Frame{Type: frameType.Enum()}}
		if frameType == sharing.V1Frame_PAIRED_KEY_ENCRYPTION {
			frame.V1.PairedKeyEncryption = &sharing.PairedKeyEncryptionFrame{SignedData: randBytes(72), SecretIdHash: randBytes(6)}
		} else {
			frame.V1.PairedKeyResult = &sharing.PairedKeyResultFrame{Status: sharing.PairedKeyResultFrame_UNABLE.Enum()}
		}
		if err := s.sendSharingFrame(frame); err != nil {
			t.Fatal(err)
		}
		smokeSharingResponse(t, s)
	}
	intro := &sharing.Frame{Version: sharing.Frame_V1.Enum(), V1: &sharing.V1Frame{Type: sharing.V1Frame_INTRODUCTION.Enum(), Introduction: &sharing.IntroductionFrame{FileMetadata: []*sharing.FileMetadata{{Name: proto.String("Verifica popup.pdf"), Type: sharing.FileMetadata_DOCUMENT.Enum(), PayloadId: proto.Int64(86420), Size: proto.Int64(1024)}}}}}
	if err := s.sendSharingFrame(intro); err != nil {
		t.Fatal(err)
	}
	t.Logf("Popup verification code: %s. No file data sent; cancellation in 20 seconds.", verificationPIN(keys.authString))
	time.Sleep(20 * time.Second)
	if err := s.sendSharingFrame(&sharing.Frame{Version: sharing.Frame_V1.Enum(), V1: &sharing.V1Frame{Type: sharing.V1Frame_CANCEL.Enum()}}); err != nil {
		t.Fatal(err)
	}
	c.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, err := readFrame(c); err != io.EOF {
		t.Fatalf("expected immediate disconnect after cancellation, got %v", err)
	}
}

func TestDeviceMulticastDiscovery(t *testing.T) {
	host := os.Getenv("REMARKABLE_TEST_HOST")
	if host == "" {
		t.Skip("opt-in device test")
	}
	iface := os.Getenv("REMARKABLE_TEST_IFACE")
	if iface == "" {
		iface = "Wi-Fi"
	}
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		t.Fatal(err)
	}
	group := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	conn, err := net.ListenMulticastUDP("udp4", ifi, group)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	p := ipv4.NewPacketConn(conn)
	p.SetMulticastInterface(ifi)
	p.SetMulticastTTL(255)
	q := new(dns.Msg)
	q.SetQuestion(serviceType+".local.", dns.TypePTR)
	q.Id = 0
	q.RecursionDesired = false
	query, _ := q.Pack()
	buf := make([]byte, 9000)
	for attempt := 0; attempt < 3; attempt++ {
		p.WriteTo(query, &ipv4.ControlMessage{IfIndex: ifi.Index}, group)
		conn.SetReadDeadline(time.Now().Add(4 * time.Second))
		for {
			n, src, err := conn.ReadFromUDP(buf)
			if err != nil {
				break
			}
			if src.IP.String() != host {
				continue
			}
			var answer dns.Msg
			if answer.Unpack(buf[:n]) != nil || !answer.Response {
				continue
			}
			for _, record := range answer.Answer {
				if record.Header().Name == serviceType+".local." {
					t.Log("multicast response from", src)
					return
				}
			}
		}
	}
	t.Fatal("no multicast response from tablet")
}

func TestDeviceFileTransfer(t *testing.T) {
	host := os.Getenv("REMARKABLE_TEST_HOST")
	if host == "" {
		t.Skip("opt-in device test")
	}
	for _, path := range []string{"../dist/QuickShare_verifica_PDF.pdf", "../dist/QuickShare_verifica_EPUB.epub"} {
		t.Run(filepath.Ext(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			c, err := net.DialTimeout("tcp", net.JoinHostPort(host, "12345"), 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(90 * time.Second))
			request := &connections.OfflineFrame{Version: connections.OfflineFrame_V1.Enum(), V1: &connections.V1Frame{Type: connections.V1Frame_CONNECTION_REQUEST.Enum(), ConnectionRequest: &connections.ConnectionRequestFrame{EndpointId: proto.String("TEST"), EndpointName: proto.String("PC verifica")}}}
			if err := writeFrame(c, marshalSmoke(t, request)); err != nil {
				t.Fatal(err)
			}
			keys := smokeClientKeys(t, c)
			t.Logf("Connection verification code: %s", verificationPIN(keys.authString))
			if err := sendConnectionResponse(c); err != nil {
				t.Fatal(err)
			}
			if _, err := readFrame(c); err != nil {
				t.Fatal(err)
			}
			s := newSession(c, keys, "")
			for _, frameType := range []sharing.V1Frame_FrameType{sharing.V1Frame_PAIRED_KEY_ENCRYPTION, sharing.V1Frame_PAIRED_KEY_RESULT} {
				frame := &sharing.Frame{Version: sharing.Frame_V1.Enum(), V1: &sharing.V1Frame{Type: frameType.Enum()}}
				if frameType == sharing.V1Frame_PAIRED_KEY_ENCRYPTION {
					frame.V1.PairedKeyEncryption = &sharing.PairedKeyEncryptionFrame{SignedData: randBytes(72), SecretIdHash: randBytes(6)}
				} else {
					frame.V1.PairedKeyResult = &sharing.PairedKeyResultFrame{Status: sharing.PairedKeyResultFrame_UNABLE.Enum()}
				}
				if err := s.sendSharingFrame(frame); err != nil {
					t.Fatal(err)
				}
				smokeSharingResponse(t, s)
			}
			payloadID := int64(13579)
			intro := &sharing.Frame{Version: sharing.Frame_V1.Enum(), V1: &sharing.V1Frame{Type: sharing.V1Frame_INTRODUCTION.Enum(), Introduction: &sharing.IntroductionFrame{FileMetadata: []*sharing.FileMetadata{{Name: proto.String(filepath.Base(path)), Type: sharing.FileMetadata_DOCUMENT.Enum(), PayloadId: proto.Int64(payloadID), Size: proto.Int64(int64(len(data)))}}}}}
			if err := s.sendSharingFrame(intro); err != nil {
				t.Fatal(err)
			}
			if result := smokeSharingResponse(t, s); result.GetV1().GetConnectionResponse().GetStatus() != sharing.ConnectionResponseFrame_ACCEPT {
				t.Fatal("rejected", result)
			}
			for offset := 0; offset <= len(data); {
				end := offset + 300
				if end > len(data) {
					end = len(data)
				}
				flags := int32(0)
				if offset == len(data) {
					flags = 1
				}
				frame := &connections.OfflineFrame{Version: connections.OfflineFrame_V1.Enum(), V1: &connections.V1Frame{Type: connections.V1Frame_PAYLOAD_TRANSFER.Enum(), PayloadTransfer: &connections.PayloadTransferFrame{PacketType: connections.PayloadTransferFrame_DATA.Enum(), PayloadHeader: &connections.PayloadTransferFrame_PayloadHeader{Id: proto.Int64(payloadID), Type: connections.PayloadTransferFrame_PayloadHeader_FILE.Enum(), TotalSize: proto.Int64(int64(len(data)))}, PayloadChunk: &connections.PayloadTransferFrame_PayloadChunk{Offset: proto.Int64(int64(offset)), Body: data[offset:end], Flags: proto.Int32(flags)}}}}
				if err := s.sendOffline(frame); err != nil {
					t.Fatal(err)
				}
				if flags == 1 {
					break
				}
				offset = end
			}
			raw, err := readFrame(c)
			if err != nil {
				t.Fatal(err)
			}
			clear, _, err := keys.decrypt(raw)
			if err != nil {
				t.Fatal(err)
			}
			var ack connections.OfflineFrame
			proto.Unmarshal(clear, &ack)
			if ack.GetV1().GetPayloadTransfer().GetPacketType() != connections.PayloadTransferFrame_PAYLOAD_ACK {
				t.Fatal("missing file acknowledgment")
			}
			t.Log(fmt.Sprintf("UKEY2 + encrypted transfer + ACK: %s (%d bytes)", path, len(data)))
		})
	}
}

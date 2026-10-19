package main

import (
	"archive/zip"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"kindshare/pb/connections"
)

func TestDocumentValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pdf")
	os.WriteFile(path, []byte("not a PDF"), 0600)
	if validateDocument(path, ".pdf") == nil {
		t.Fatal("invalid PDF accepted")
	}
	os.WriteFile(path, []byte("%PDF-1.4\n"), 0600)
	if err := validateDocument(path, ".PDF"); err != nil {
		t.Fatal(err)
	}
	ep := filepath.Join(dir, "test.epub")
	f, _ := os.Create(ep)
	z := zip.NewWriter(f)
	w, _ := z.Create("mimetype")
	w.Write([]byte("application/epub+zip"))
	z.Close()
	f.Close()
	if err := validateDocument(ep, ".epub"); err != nil {
		t.Fatal(err)
	}
	if validateDocument(path, ".exe") == nil {
		t.Fatal("unsupported type accepted")
	}
}

func TestNativeUploadStreamsWithContentLength(t *testing.T) {
	data := bytes.Repeat([]byte("%PDF-1.4\n"), 10000)
	path := filepath.Join(t.TempDir(), "Libro con spazi.pdf")
	os.WriteFile(path, data, 0600)
	listed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/documents/":
			listed = true
			w.Write([]byte("[]"))
		case "/upload":
			if !listed {
				t.Error("root not selected")
			}
			if r.ContentLength <= int64(len(data)) || len(r.TransferEncoding) != 0 {
				t.Error("upload not fixed length")
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			f, header, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				http.Error(w, "bad multipart", 400)
				return
			}
			defer f.Close()
			if !strings.HasPrefix(header.Header.Get("Content-Disposition"), "form-data; name=\"file\";") {
				t.Error("Xochitl requires quoted multipart attributes")
			}
			got, _ := io.ReadAll(f)
			if header.Filename != filepath.Base(path) || !bytes.Equal(got, data) {
				t.Error("file corrupted")
			}
		default:
			t.Error("unexpected path")
		}
	}))
	defer server.Close()
	if err := nativeUpload(server.Client(), server.URL, path); err != nil {
		t.Fatal(err)
	}
}

func TestInterruptedAndMalformedFileNeverBecomesImportable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.pdf")
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	s := newSession(a, nil, dir)
	s.files[42] = &incomingFile{name: "file.pdf", size: 16, path: path, partial: path + ".partial"}
	pt := &connections.PayloadTransferFrame{PayloadHeader: &connections.PayloadTransferFrame_PayloadHeader{Id: proto.Int64(42), Type: connections.PayloadTransferFrame_PayloadHeader_FILE.Enum()}, PayloadChunk: &connections.PayloadTransferFrame_PayloadChunk{Offset: proto.Int64(7), Body: []byte("bad")}}
	if s.handlePayload(pt) == nil {
		t.Fatal("out of order chunk accepted")
	}
	s.cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("partial file promoted")
	}
	if _, err := os.Stat(path + ".partial"); !os.IsNotExist(err) {
		t.Fatal("partial file leaked")
	}
}

func TestFailedNativeImportPreservesQueue(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "book.pdf")
	os.WriteFile(path, []byte("%PDF-1.4\n"), 0600)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "busy", 503) }))
	defer server.Close()
	if importOne(server.Client(), server.URL, root, path) == nil {
		t.Fatal("failure accepted")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("pending file lost")
	}
}

func TestReconcileNativeTitleAndDeletedDocument(t *testing.T) {
	lib := t.TempDir()
	input := filepath.Join(t.TempDir(), "file.epub")
	data := []byte("immutable original")
	os.WriteFile(input, data, 0600)
	os.WriteFile(filepath.Join(lib, "native-id.epub"), data, 0600)
	os.WriteFile(filepath.Join(lib, "native-id.metadata"), []byte(`{"visibleName":"Different EPUB book title","deleted":false,"parent":""}`), 0600)
	hash, _ := fileHash(input)
	if id := findImportedIn(lib, input, hash); id != "native-id" {
		t.Fatal("native title must not prevent reconciliation")
	}
	os.WriteFile(filepath.Join(lib, "native-id.metadata"), []byte(`{"visibleName":"Different EPUB book title","deleted":true}`), 0600)
	if id := findImportedIn(lib, input, hash); id != "" {
		t.Fatal("deleted document must allow reimport")
	}
}

func TestRecoverOnlyIncompleteSpoolFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "transfer-test")
	os.Mkdir(dir, 0700)
	partial := filepath.Join(dir, "book.pdf.partial")
	complete := filepath.Join(dir, "saved.pdf")
	os.WriteFile(partial, []byte("incomplete"), 0600)
	os.WriteFile(complete, []byte("complete"), 0600)
	recoverInbox(root)
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatal("crashed partial survived")
	}
	if _, err := os.Stat(complete); err != nil {
		t.Fatal("completed queue file lost")
	}
}

func TestOversizedKeyCoordinateDoesNotPanic(t *testing.T) {
	if b := pad32(bytes.Repeat([]byte{255}, 33)); b != nil {
		t.Fatal("invalid coordinate accepted")
	}
}

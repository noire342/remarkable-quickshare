package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var importWake = make(chan struct{}, 1)

const libraryPath = "/home/root/.local/share/remarkable/xochitl"

func queueImport() {
	select {
	case importWake <- struct{}{}:
	default:
	}
}

func validateDocument(path, extension string) error {
	switch strings.ToLower(extension) {
	case ".pdf":
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		var b [1024]byte
		n, _ := f.Read(b[:])
		if !bytes.Contains(b[:n], []byte("%PDF-")) {
			return fmt.Errorf("invalid PDF header")
		}
	case ".epub":
		z, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		defer z.Close()
		if len(z.File) > 10000 {
			return fmt.Errorf("too many EPUB entries")
		}
		var expanded uint64
		valid := false
		for _, entry := range z.File {
			if entry.UncompressedSize64 > 500000000-expanded {
				return fmt.Errorf("EPUB expands beyond limit")
			}
			expanded += entry.UncompressedSize64
			if entry.Name == "mimetype" {
				r, e := entry.Open()
				if e != nil {
					return e
				}
				b, e := io.ReadAll(io.LimitReader(r, 128))
				r.Close()
				valid = e == nil && string(b) == "application/epub+zip"
			}
		}
		if !valid {
			return fmt.Errorf("invalid EPUB mimetype")
		}
	default:
		return fmt.Errorf("only PDF and EPUB are supported")
	}
	return nil
}

type receipt struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SHA256   string `json:"sha256"`
	Imported string `json:"imported"`
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Only immutable original files and metadata are read. Xochitl alone writes the library.
// This also reconciles an import interrupted after HTTP success but before queue cleanup.
func findImported(path, hash string) string {
	return findImportedIn(libraryPath, path, hash)
}

func findImportedIn(libraryPath, path, hash string) string {
	input, err := os.Stat(path)
	if err != nil {
		return ""
	}
	entries, _ := os.ReadDir(libraryPath)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".metadata") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(libraryPath, entry.Name()))
		if err != nil {
			continue
		}
		var meta struct {
			VisibleName string `json:"visibleName"`
			Deleted     bool   `json:"deleted"`
			Parent      string `json:"parent"`
		}
		if json.Unmarshal(b, &meta) != nil || meta.Deleted || meta.Parent == "trash" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".metadata")
		for _, ext := range []string{".pdf", ".epub"} {
			original := filepath.Join(libraryPath, id+ext)
			stat, err := os.Stat(original)
			if err != nil || stat.Size() != input.Size() {
				continue
			}
			if actual, err := fileHash(original); err == nil && actual == hash {
				return id
			}
		}
	}
	return ""
}

func nativeUpload(client *http.Client, base, path string) error {
	// Explicitly select My Files; the API otherwise imports into whichever folder the web UI last listed.
	resp, err := client.Get(base + "/documents/")
	if err != nil {
		return err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<20))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("library HTTP %d", resp.StatusCode)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var envelope bytes.Buffer
	form := multipart.NewWriter(&envelope)
	contentType := form.FormDataContentType()
	header := make(textproto.MIMEHeader)
	// Xochitl's multipart parser requires quoted attributes, even for token-safe names.
	filename := strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(filepath.Base(path))
	header.Set("Content-Disposition", fmt.Sprintf("form-data; name=\"file\"; filename=\"%s\"", filename))
	t := "application/pdf"
	if strings.EqualFold(filepath.Ext(path), ".epub") {
		t = "application/epub+zip"
	}
	header.Set("Content-Type", t)
	if _, err := form.CreatePart(header); err != nil {
		return err
	}
	prefix := append([]byte(nil), envelope.Bytes()...)
	envelope.Reset()
	if err := form.Close(); err != nil {
		return err
	}
	suffix := append([]byte(nil), envelope.Bytes()...)
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	reader := io.MultiReader(bytes.NewReader(prefix), f, bytes.NewReader(suffix))
	req, err := http.NewRequest(http.MethodPost, base+"/upload", reader)
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(prefix)) + stat.Size() + int64(len(suffix))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", base)
	req.Header.Set("Referer", base+"/")
	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}

func importOne(client *http.Client, base, root, path string) error {
	if err := validateDocument(path, filepath.Ext(path)); err != nil {
		return err
	}
	hash, err := fileHash(path)
	if err != nil {
		return err
	}
	id := findImported(path, hash)
	if id == "" {
		if err := nativeUpload(client, base, path); err != nil {
			return err
		}
		for i := 0; i < 30; i++ {
			id = findImported(path, hash)
			if id != "" {
				break
			}
			time.Sleep(time.Second)
		}
		if id == "" {
			return fmt.Errorf("Xochitl has not registered the imported file yet")
		}
	}
	record := receipt{id, filepath.Base(path), hash, time.Now().UTC().Format(time.RFC3339)}
	if err := os.MkdirAll(filepath.Join(root, "receipts"), 0700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(record, "", "  ")
	receiptPath := filepath.Join(root, "receipts", hash+".json")
	if err := os.WriteFile(receiptPath+".tmp", b, 0600); err != nil {
		return err
	}
	if err := os.Rename(receiptPath+".tmp", receiptPath); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	os.Remove(filepath.Dir(path))
	log.Printf("IMPORTED %q in Xochitl (%s, SHA256 %s)", record.Name, id, hash)
	filesImported.Add(1)
	return nil
}

// Run before the listener starts: no live transfer can own a partial file yet.
func recoverInbox(root string) {
	os.MkdirAll(root, 0700)
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "transfer-") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		files, _ := os.ReadDir(dir)
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".partial") {
				os.Remove(filepath.Join(dir, file.Name()))
			}
		}
		os.Remove(dir)
	}
}

func runImporter(root, base string) {
	base = strings.TrimRight(base, "/")
	client := &http.Client{Timeout: 3 * time.Minute, Transport: &http.Transport{Proxy: nil, MaxIdleConns: 1, IdleConnTimeout: 30 * time.Second}}
	// One worker, streaming bodies, and a RAM-only retry timer keep idle cost low.
	timer := time.NewTicker(30 * time.Second)
	defer timer.Stop()
	for {
		lastError := ""
		filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() && entry.Name() == "receipts" {
				return filepath.SkipDir
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !entry.IsDir() && (ext == ".pdf" || ext == ".epub") {
				if err := importOne(client, base, root, path); err != nil {
					lastError = filepath.Base(path) + ": " + err.Error()
					log.Printf("import pending %q: %v", filepath.Base(path), err)
				}
			}
			return nil
		})
		lastImportError.Store(lastError)
		select {
		case <-importWake:
		case <-timer.C:
		}
	}
}

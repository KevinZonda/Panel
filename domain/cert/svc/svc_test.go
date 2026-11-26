package svc_test

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KevinZonda/Panel/domain/cert/svc"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/models/apimod"
)

func setup(t *testing.T) (*svc.Service, string, string) {
	t.Helper()
	root := t.TempDir()
	publicDir := filepath.Join(root, "cert")
	selfSignDir := filepath.Join(root, "cert-self-sign")
	return svc.NewService(publicDir, selfSignDir), publicDir, selfSignDir
}

func TestGenerateSelfSignCreatesValidChain(t *testing.T) {
	service, _, selfSignDir := setup(t)

	_, err := service.GenerateSelfSign("example.com", []string{"example.com", "www.example.com"}, 365)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	for _, file := range []string{"ca.crt", "server.crt", "server.key"} {
		if _, err := os.Stat(filepath.Join(selfSignDir, "example.com", file)); err != nil {
			t.Fatalf("missing file %s: %v", file, err)
		}
	}

	// key must not be world-readable
	info, err := os.Stat(filepath.Join(selfSignDir, "example.com", "server.key"))
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected key perm 0600, got %o", info.Mode().Perm())
	}

	// server cert must verify against the generated CA
	readCert := func(file string) *x509.Certificate {
		raw, err := os.ReadFile(filepath.Join(selfSignDir, "example.com", file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		block, _ := pem.Decode(raw)
		if block == nil {
			t.Fatalf("%s is not PEM", file)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		return cert
	}
	ca := readCert("ca.crt")
	server := readCert("server.crt")

	if !ca.IsCA {
		t.Fatal("ca.crt is not a CA")
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, err := server.Verify(x509.VerifyOptions{
		Roots:     roots,
		DNSName:   "www.example.com",
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("server cert does not verify against CA: %v", err)
	}
	if server.Subject.CommonName != "example.com" {
		t.Fatalf("unexpected CN: %q", server.Subject.CommonName)
	}
	if len(server.DNSNames) != 2 {
		t.Fatalf("expected 2 SANs, got %v", server.DNSNames)
	}
	if got := int(time.Until(server.NotAfter).Hours() / 24); got < 360 || got > 366 {
		t.Fatalf("unexpected validity: %d days left", got)
	}
}

func TestGenerateSelfSignConflict(t *testing.T) {
	service, _, _ := setup(t)
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 30); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 30); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestGenerateSelfSignRejectsBadInput(t *testing.T) {
	service, _, _ := setup(t)
	if _, err := service.GenerateSelfSign("../evil", []string{"example.com"}, 30); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for path traversal, got %v", err)
	}
	if _, err := service.GenerateSelfSign("example.com", nil, 30); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for empty dns names, got %v", err)
	}
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 0); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for zero days, got %v", err)
	}
}

func TestListAndGet(t *testing.T) {
	service, publicDir, _ := setup(t)

	if _, err := service.GenerateSelfSign("self.example.com", []string{"self.example.com"}, 30); err != nil {
		t.Fatalf("generate: %v", err)
	}

	// upload a public cert: reuse generated files as content
	caRaw, err := os.ReadFile(filepath.Join(publicDir, "..", "cert-self-sign", "self.example.com", "ca.crt"))
	if err != nil {
		t.Fatalf("read generated ca: %v", err)
	}
	if err := service.UploadFile(apimod.CertTypePublic, "pub.example.com", "pub", string(caRaw)); err != nil {
		t.Fatalf("upload pub: %v", err)
	}

	list, err := service.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 certs, got %d", len(list.Items))
	}

	detail, err := service.Get(apimod.CertTypeSelfSign, "self.example.com")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if detail.CommonName != "self.example.com" {
		t.Fatalf("unexpected CN: %q", detail.CommonName)
	}
	if detail.Expired {
		t.Fatal("fresh cert should not be expired")
	}
	if len(detail.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(detail.Files))
	}
	var keyFile *apimod.CertFileInfo
	for i := range detail.Files {
		if detail.Files[i].Name == "server.key" {
			keyFile = &detail.Files[i]
		}
	}
	if keyFile == nil || !keyFile.IsPrivate {
		t.Fatal("server.key must be marked private")
	}
}

func TestUploadValidation(t *testing.T) {
	service, _, selfSignDir := setup(t)

	if err := service.UploadFile(apimod.CertTypePublic, "example.com", "pub", "not pem at all"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for garbage content, got %v", err)
	}
	if err := service.UploadFile(apimod.CertTypePublic, "example.com", "evil", "x"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for unknown file, got %v", err)
	}
	if err := service.UploadFile(apimod.CertTypeSelfSign, "example.com", "ca.key", "x"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for disallowed selfsign file, got %v", err)
	}

	// a certificate is not a valid private key
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 30); err != nil {
		t.Fatalf("generate: %v", err)
	}
	caRaw, _ := os.ReadFile(filepath.Join(selfSignDir, "example.com", "ca.crt"))
	if err := service.UploadFile(apimod.CertTypeSelfSign, "example.com", "server.key", string(caRaw)); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error when uploading cert as key, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	service, _, _ := setup(t)
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 30); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := service.Delete(apimod.CertTypeSelfSign, "example.com"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := service.Get(apimod.CertTypeSelfSign, "example.com"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
	if err := service.Delete(apimod.CertTypeSelfSign, "example.com"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected not found deleting twice, got %v", err)
	}
}

func TestListIgnoresMissingDirs(t *testing.T) {
	service, _, _ := setup(t)
	list, err := service.List()
	if err != nil {
		t.Fatalf("list with missing dirs should not error: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("expected empty list, got %d items", len(list.Items))
	}
}

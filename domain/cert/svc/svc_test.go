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

func readCertFile(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatalf("%s is not PEM", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return cert
}

func TestGenerateSelfSignCreatesValidChain(t *testing.T) {
	service, _, selfSignDir := setup(t)

	_, err := service.GenerateSelfSign("example.com", []string{"example.com", "www.example.com"}, 365, svc.SelfSignSubject{})
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

	if ca.Subject.CommonName != svc.DefaultCACommonName {
		t.Fatalf("ca CN = %q, want %q", ca.Subject.CommonName, svc.DefaultCACommonName)
	}
	if len(ca.Subject.Country) != 1 || ca.Subject.Country[0] != "GB" {
		t.Fatalf("ca country = %v, want [GB]", ca.Subject.Country)
	}
	if len(ca.Subject.Organization) != 1 || ca.Subject.Organization[0] != "KevinZonda Research" {
		t.Fatalf("ca org = %v", ca.Subject.Organization)
	}
	if server.Subject.CommonName != "example.com" {
		t.Fatalf("server CN = %q", server.Subject.CommonName)
	}
	if len(server.Subject.OrganizationalUnit) != 1 || server.Subject.OrganizationalUnit[0] != "One-Time Certificate" {
		t.Fatalf("server OU = %v", server.Subject.OrganizationalUnit)
	}

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
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 30, svc.SelfSignSubject{}); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 30, svc.SelfSignSubject{}); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestGenerateSelfSignRejectsBadInput(t *testing.T) {
	service, _, _ := setup(t)
	if _, err := service.GenerateSelfSign("../evil", []string{"example.com"}, 30, svc.SelfSignSubject{}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for path traversal, got %v", err)
	}
	if _, err := service.GenerateSelfSign("example.com", nil, 30, svc.SelfSignSubject{}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for empty dns names, got %v", err)
	}
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, -1, svc.SelfSignSubject{}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for negative days, got %v", err)
	}
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 3651, svc.SelfSignSubject{}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for days above max, got %v", err)
	}
}

func TestListAndGet(t *testing.T) {
	service, publicDir, _ := setup(t)

	if _, err := service.GenerateSelfSign("self.example.com", []string{"self.example.com"}, 30, svc.SelfSignSubject{}); err != nil {
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
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 30, svc.SelfSignSubject{}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	caRaw, _ := os.ReadFile(filepath.Join(selfSignDir, "example.com", "ca.crt"))
	if err := service.UploadFile(apimod.CertTypeSelfSign, "example.com", "server.key", string(caRaw)); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error when uploading cert as key, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	service, _, _ := setup(t)
	if _, err := service.GenerateSelfSign("example.com", []string{"example.com"}, 30, svc.SelfSignSubject{}); err != nil {
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

func TestGenerateSelfSignCustomSubject(t *testing.T) {
	service, _, selfSignDir := setup(t)

	subject := svc.SelfSignSubject{
		Country:      "US",
		Province:     "California",
		Locality:     "San Francisco",
		Organization: "Example Inc",
		OrgUnit:      "Infra",
		CACommonName: "Example Root CA",
	}
	_, err := service.GenerateSelfSign("custom.com", []string{"custom.com"}, 30, subject)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	caCert := readCertFile(t, filepath.Join(selfSignDir, "custom.com", "ca.crt"))
	if caCert.Subject.CommonName != "Example Root CA" {
		t.Fatalf("ca CN = %q", caCert.Subject.CommonName)
	}
	if caCert.Subject.Country[0] != "US" || caCert.Subject.Province[0] != "California" || caCert.Subject.Locality[0] != "San Francisco" {
		t.Fatalf("ca subject = %+v", caCert.Subject)
	}

	// empty subject falls back to defaults
	_, err = service.GenerateSelfSign("defaulted.com", []string{"defaulted.com"}, 0, svc.SelfSignSubject{})
	if err != nil {
		t.Fatalf("generate defaulted: %v", err)
	}
	serverCert := readCertFile(t, filepath.Join(selfSignDir, "defaulted.com", "server.crt"))
	if serverCert.Subject.Organization[0] != svc.DefaultOrganization {
		t.Fatalf("default org = %q", serverCert.Subject.Organization[0])
	}
	// days=0 falls back to the 3650-day default
	wantExpiry := time.Now().AddDate(0, 0, svc.DefaultValidDays)
	if delta := serverCert.NotAfter.Sub(wantExpiry); delta < -time.Hour || delta > time.Hour {
		t.Fatalf("default validity NotAfter = %v, want ~%v", serverCert.NotAfter, wantExpiry)
	}
}

func TestCreatePublicCert(t *testing.T) {
	service, publicDir, selfSignDir := setup(t)

	// generate key material to upload: reuse a self-signed cert
	if _, err := service.GenerateSelfSign("donor.com", []string{"donor.com", "www.donor.com"}, 30, svc.SelfSignSubject{}); err != nil {
		t.Fatalf("generate donor: %v", err)
	}
	pub, err := os.ReadFile(filepath.Join(selfSignDir, "donor.com", "server.crt"))
	if err != nil {
		t.Fatalf("read donor pub: %v", err)
	}
	priv, err := os.ReadFile(filepath.Join(selfSignDir, "donor.com", "server.key"))
	if err != nil {
		t.Fatalf("read donor priv: %v", err)
	}

	// happy path
	if _, err := service.CreatePublicCert("ext.com", string(pub), string(priv)); err != nil {
		t.Fatalf("create public: %v", err)
	}
	writtenPub, err := os.ReadFile(filepath.Join(publicDir, "ext.com", "pub"))
	if err != nil {
		t.Fatalf("read written pub: %v", err)
	}
	if string(writtenPub) != string(pub) {
		t.Fatal("written pub differs from upload")
	}
	info, err := os.Stat(filepath.Join(publicDir, "ext.com", "priv"))
	if err != nil {
		t.Fatalf("stat priv: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected priv perm 0600, got %o", info.Mode().Perm())
	}

	// appears in listing with parsed metadata
	list, err := service.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, item := range list.Items {
		if item.Type == apimod.CertTypePublic && item.Name == "ext.com" {
			found = true
			if item.CommonName != "donor.com" {
				t.Fatalf("listed CN = %q", item.CommonName)
			}
		}
	}
	if !found {
		t.Fatal("ext.com not in list")
	}

	// conflict
	if _, err := service.CreatePublicCert("ext.com", string(pub), string(priv)); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}

	// mismatched key rejected: use a different cert's key
	if _, err := service.GenerateSelfSign("other.com", []string{"other.com"}, 30, svc.SelfSignSubject{}); err != nil {
		t.Fatalf("generate other: %v", err)
	}
	otherKey, err := os.ReadFile(filepath.Join(selfSignDir, "other.com", "server.key"))
	if err != nil {
		t.Fatalf("read other key: %v", err)
	}
	if _, err := service.CreatePublicCert("mismatch.com", string(pub), string(otherKey)); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for mismatched key, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(publicDir, "mismatch.com")); !os.IsNotExist(err) {
		t.Fatal("mismatch.com directory should not exist after rejection")
	}

	// garbage rejected
	if _, err := service.CreatePublicCert("garbage.com", "not pem", string(priv)); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for bad pub, got %v", err)
	}
	if _, err := service.CreatePublicCert("garbage.com", string(pub), "not pem"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for bad priv, got %v", err)
	}

	// bad name rejected
	if _, err := service.CreatePublicCert("../evil", string(pub), string(priv)); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for bad name, got %v", err)
	}
}

func TestCreatePublicCertChainValidation(t *testing.T) {
	service, publicDir, selfSignDir := setup(t)

	if _, err := service.GenerateSelfSign("donor.com", []string{"donor.com"}, 30, svc.SelfSignSubject{}); err != nil {
		t.Fatalf("generate donor: %v", err)
	}
	read := func(domain, f string) string {
		b, err := os.ReadFile(filepath.Join(selfSignDir, domain, f))
		if err != nil {
			t.Fatalf("read %s/%s: %v", domain, f, err)
		}
		return string(b)
	}
	leaf, ca, key := read("donor.com", "server.crt"), read("donor.com", "ca.crt"), read("donor.com", "server.key")

	// fullchain leaf+ca accepted, written verbatim
	fullchain := leaf + ca
	if _, err := service.CreatePublicCert("chain.com", fullchain, key); err != nil {
		t.Fatalf("fullchain upload should be accepted: %v", err)
	}
	written, err := os.ReadFile(filepath.Join(publicDir, "chain.com", "pub"))
	if err != nil {
		t.Fatalf("read written: %v", err)
	}
	if string(written) != fullchain {
		t.Fatal("fullchain not written verbatim")
	}

	// metadata uses the leaf
	list, err := service.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, item := range list.Items {
		if item.Name == "chain.com" && item.CommonName != "donor.com" {
			t.Fatalf("chain.com listed CN = %q, want leaf donor.com", item.CommonName)
		}
	}

	// garbage tail rejected
	if _, err := service.CreatePublicCert("garbage-tail.com", leaf+"trailing garbage", key); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for garbage tail, got %v", err)
	}

	// wrong order rejected with chain error
	if _, err := service.CreatePublicCert("wrong-order.com", ca+leaf, key); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for wrong order, got %v", err)
	}

	// chain missing its link rejected: leaf signed by CA, appended unrelated CA
	if _, err := service.GenerateSelfSign("unrelated.com", []string{"unrelated.com"}, 30, svc.SelfSignSubject{}); err != nil {
		t.Fatalf("generate unrelated: %v", err)
	}
	unrelatedCA := read("unrelated.com", "ca.crt")
	if _, err := service.CreatePublicCert("broken-chain.com", leaf+unrelatedCA, key); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for broken chain, got %v", err)
	}

	// key pasted into pub rejected
	if _, err := service.CreatePublicCert("key-in-pub.com", key, key); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for key in pub, got %v", err)
	}

	// leaf-only still fine
	if _, err := service.CreatePublicCert("leaf-only.com", leaf, key); err != nil {
		t.Fatalf("leaf-only upload should be accepted: %v", err)
	}
}

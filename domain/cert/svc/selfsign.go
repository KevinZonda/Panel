package svc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"time"

	"github.com/KevinZonda/Panel/domain/cert/dao"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/models/apimod"
)

// SelfSignSubject carries the x509 subject fields for generated
// certificates. Empty fields fall back to the KevinZonda conventions.
type SelfSignSubject struct {
	Country      string
	Province     string
	Locality     string
	Organization string
	OrgUnit      string
	CACommonName string
}

const (
	DefaultCountry      = "GB"
	DefaultProvince     = "Greater London"
	DefaultLocality     = "London"
	DefaultOrganization = "KevinZonda Research"
	DefaultOrgUnit      = "One-Time Certificate"
	DefaultCACommonName = "KevinZonda One-Time Certificate Authority"
	DefaultValidDays    = 3650
)

func (s *SelfSignSubject) withDefaults() {
	if s.Country == "" {
		s.Country = DefaultCountry
	}
	if s.Province == "" {
		s.Province = DefaultProvince
	}
	if s.Locality == "" {
		s.Locality = DefaultLocality
	}
	if s.Organization == "" {
		s.Organization = DefaultOrganization
	}
	if s.OrgUnit == "" {
		s.OrgUnit = DefaultOrgUnit
	}
	if s.CACommonName == "" {
		s.CACommonName = DefaultCACommonName
	}
}

func (s *SelfSignSubject) pkixName(commonName string) pkix.Name {
	return pkix.Name{
		CommonName:         commonName,
		Country:            []string{s.Country},
		Province:           []string{s.Province},
		Locality:           []string{s.Locality},
		Organization:       []string{s.Organization},
		OrganizationalUnit: []string{s.OrgUnit},
	}
}

// GenerateSelfSign creates a fresh CA plus a server certificate signed by
// it, writing ca.crt, server.crt and server.key into the domain directory.
func (s *Service) GenerateSelfSign(name string, dnsNames []string, days int, subject SelfSignSubject) (*apimod.CertMutationResult, error) {
	if !dao.ValidName(name) {
		return nil, apperr.Validationf("cert name is invalid")
	}
	cleaned, err := validateDNSNames(dnsNames)
	if err != nil {
		return nil, err
	}
	if days == 0 {
		days = DefaultValidDays
	}
	if days < 1 || days > 3650 {
		return nil, apperr.Validationf("days must be between 1 and 3650")
	}
	subject.withDefaults()

	root := s.selfSignDir
	if dao.DomainExists(root, name) {
		return nil, apperr.Conflictf("cert already exists")
	}

	caKey, caCertPEM, caCert, err := generateCA(subject)
	if err != nil {
		return nil, err
	}
	serverCertPEM, serverKeyPEM, err := generateServerCert(caCert, caKey, cleaned, days, subject)
	if err != nil {
		return nil, err
	}

	// Write CA artifacts first; if any write fails midway the directory is
	// removed so we never leave a half-usable cert behind.
	written := false
	defer func() {
		if !written {
			_ = dao.DeleteDomain(root, name)
		}
	}()

	if err := dao.WriteFile(root, name, "ca.crt", caCertPEM, 0o644); err != nil {
		return nil, apperr.Dependencyf("write ca.crt failed: %v", err)
	}
	if err := dao.WriteFile(root, name, "server.crt", serverCertPEM, 0o644); err != nil {
		return nil, apperr.Dependencyf("write server.crt failed: %v", err)
	}
	if err := dao.WriteFile(root, name, "server.key", serverKeyPEM, 0o600); err != nil {
		return nil, apperr.Dependencyf("write server.key failed: %v", err)
	}
	written = true

	return &apimod.CertMutationResult{Name: name, Type: apimod.CertTypeSelfSign}, nil
}

func validateDNSNames(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, apperr.Validationf("dns_names must not be empty")
	}
	if len(names) > 32 {
		return nil, apperr.Validationf("dns_names supports at most 32 entries")
	}
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(strings.ToLower(n))
		if n == "" {
			continue
		}
		if len(n) > 253 || strings.ContainsAny(n, "/\\ \t\r\n") {
			return nil, apperr.Validationf("dns name %q is invalid", n)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		cleaned = append(cleaned, n)
	}
	if len(cleaned) == 0 {
		return nil, apperr.Validationf("dns_names must not be empty")
	}
	return cleaned, nil
}

func generateCA(subject SelfSignSubject) (*ecdsa.PrivateKey, []byte, *x509.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               subject.pkixName(subject.CACommonName),
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, err
	}
	return key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), cert, nil
}

func generateServerCert(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, dnsNames []string, days int, subject SelfSignSubject) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               subject.pkixName(dnsNames[0]),
		DNSNames:              dnsNames,
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(0, 0, days),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}

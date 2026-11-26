package svc

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/KevinZonda/Panel/domain/cert/dao"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/models/apimod"
)

const maxCertFileBytes = 1024 * 1024

type Service struct {
	publicDir   string
	selfSignDir string
}

func NewService(publicDir, selfSignDir string) *Service {
	return &Service{publicDir: publicDir, selfSignDir: selfSignDir}
}

func ValidType(certType string) bool {
	return certType == apimod.CertTypePublic || certType == apimod.CertTypeSelfSign
}

func (s *Service) rootFor(certType string) (string, error) {
	switch certType {
	case apimod.CertTypePublic:
		return s.publicDir, nil
	case apimod.CertTypeSelfSign:
		return s.selfSignDir, nil
	}
	return "", apperr.Validationf("cert type is invalid")
}

// primaryFile is the certificate file parsed for x509 details.
func primaryFile(certType string) string {
	if certType == apimod.CertTypePublic {
		return "pub"
	}
	return "server.crt"
}

func privateFiles(certType string) map[string]bool {
	if certType == apimod.CertTypePublic {
		return map[string]bool{"priv": true}
	}
	return map[string]bool{"server.key": true}
}

func (s *Service) List() (*apimod.CertList, error) {
	var items []apimod.CertSummary
	for _, certType := range []string{apimod.CertTypePublic, apimod.CertTypeSelfSign} {
		root, _ := s.rootFor(certType)
		domains, err := dao.ListDomains(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, domain := range domains {
			items = append(items, s.summary(certType, root, domain))
		}
	}
	return &apimod.CertList{Items: items}, nil
}

func (s *Service) summary(certType, root, domain string) apimod.CertSummary {
	item := apimod.CertSummary{Name: domain, Type: certType}
	content, err := dao.ReadFile(root, domain, primaryFile(certType))
	if err != nil {
		item.ParseError = "primary certificate file missing"
		return item
	}
	cert, err := parseCertificate(content)
	if err != nil {
		item.ParseError = err.Error()
		return item
	}
	fillSummary(&item, cert)
	return item
}

func (s *Service) Get(certType, name string) (*apimod.CertDetail, error) {
	if !ValidType(certType) {
		return nil, apperr.Validationf("cert type is invalid")
	}
	if !dao.ValidName(name) {
		return nil, apperr.Validationf("cert name is invalid")
	}
	root, _ := s.rootFor(certType)
	if !dao.DomainExists(root, name) {
		return nil, apperr.NotFoundf("cert not found")
	}

	sum := s.summary(certType, root, name)
	detail := &apimod.CertDetail{CertSummary: sum}

	content, err := dao.ReadFile(root, name, primaryFile(certType))
	if err == nil {
		if cert, parseErr := parseCertificate(content); parseErr == nil {
			detail.SerialNumber = cert.SerialNumber.String()
			detail.SignatureAlg = cert.SignatureAlgorithm.String()
		}
	}

	files, err := dao.ListFiles(root, name)
	if err != nil {
		return nil, err
	}
	priv := privateFiles(certType)
	for _, f := range files {
		detail.Files = append(detail.Files, apimod.CertFileInfo{
			Name:       f.Name,
			SizeBytes:  f.SizeBytes,
			ModifiedAt: f.ModifiedAt,
			IsPrivate:  priv[f.Name],
		})
	}
	return detail, nil
}

func (s *Service) ReadFile(certType, name, file string) ([]byte, bool, error) {
	if !ValidType(certType) {
		return nil, false, apperr.Validationf("cert type is invalid")
	}
	if !dao.ValidName(name) || !dao.ValidName(file) {
		return nil, false, apperr.Validationf("cert or file name is invalid")
	}
	root, _ := s.rootFor(certType)
	if !dao.DomainExists(root, name) {
		return nil, false, apperr.NotFoundf("cert not found")
	}
	content, err := dao.ReadFile(root, name, file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, apperr.NotFoundf("cert file not found")
		}
		return nil, false, err
	}
	return content, privateFiles(certType)[file], nil
}

// CreatePublicCert adds an externally issued certificate to the public
// store, writing pub and priv into a fresh domain directory. The private
// key must match the certificate's public key; a partial write removes
// the whole directory so no half-installed cert is left behind.
func (s *Service) CreatePublicCert(name, pub, priv string) (*apimod.CertMutationResult, error) {
	if !dao.ValidName(name) {
		return nil, apperr.Validationf("cert name is invalid")
	}
	if len(pub) == 0 || len(pub) > maxCertFileBytes {
		return nil, apperr.Validationf("pub must be 1-%d bytes", maxCertFileBytes)
	}
	if len(priv) == 0 || len(priv) > maxCertFileBytes {
		return nil, apperr.Validationf("priv must be 1-%d bytes", maxCertFileBytes)
	}
	if err := validatePEM("pub", pub); err != nil {
		return nil, err
	}
	if err := validatePEM("priv", priv); err != nil {
		return nil, err
	}
	chain, err := parseCertificateChain(pub)
	if err != nil {
		return nil, err
	}
	if err := validateKeyMatchesCert(chain[0], priv); err != nil {
		return nil, err
	}

	root := s.publicDir
	if dao.DomainExists(root, name) {
		return nil, apperr.Conflictf("cert already exists")
	}

	written := false
	defer func() {
		if !written {
			_ = dao.DeleteDomain(root, name)
		}
	}()
	if err := dao.WriteFile(root, name, "pub", []byte(pub), 0o644); err != nil {
		return nil, apperr.Dependencyf("write pub failed: %v", err)
	}
	if err := dao.WriteFile(root, name, "priv", []byte(priv), 0o600); err != nil {
		return nil, apperr.Dependencyf("write priv failed: %v", err)
	}
	written = true
	return &apimod.CertMutationResult{Name: name, Type: apimod.CertTypePublic}, nil
}

// validateKeyMatchesCert parses the PEM private key and rejects it unless
// its public key equals the certificate's public key.
func validateKeyMatchesCert(cert *x509.Certificate, privPEM string) error {
	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		return apperr.Validationf("private key is not valid PEM")
	}
	keyPub, err := privateKeyPublicKey(block.Bytes)
	if err != nil {
		return apperr.Validationf("private key does not parse")
	}
	certPubDER, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return apperr.Validationf("certificate public key does not parse")
	}
	keyPubDER, err := x509.MarshalPKIXPublicKey(keyPub)
	if err != nil {
		return apperr.Validationf("private key public key does not parse")
	}
	if !bytes.Equal(certPubDER, keyPubDER) {
		return apperr.Validationf("private key does not match certificate")
	}
	return nil
}

func privateKeyPublicKey(der []byte) (any, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key.Public(), nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if signer, ok := key.(crypto.Signer); ok {
			return signer.Public(), nil
		}
		return nil, fmt.Errorf("unsupported pkcs8 key type")
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key.Public(), nil
	}
	return nil, fmt.Errorf("private key does not parse")
}

func (s *Service) UploadFile(certType, name, file, content string) error {
	if !ValidType(certType) {
		return apperr.Validationf("cert type is invalid")
	}
	if !dao.ValidName(name) {
		return apperr.Validationf("cert name is invalid")
	}
	if err := validateFileName(certType, file); err != nil {
		return err
	}
	if len(content) == 0 {
		return apperr.Validationf("content must not be empty")
	}
	if len(content) > maxCertFileBytes {
		return apperr.Validationf("content exceeds %d bytes", maxCertFileBytes)
	}
	if err := validatePEM(file, content); err != nil {
		return err
	}

	root, _ := s.rootFor(certType)
	perm := os.FileMode(0o644)
	if privateFiles(certType)[file] {
		perm = 0o600
	}
	if err := dao.WriteFile(root, name, file, []byte(content), perm); err != nil {
		return apperr.Dependencyf("write cert file failed: %v", err)
	}
	return nil
}

func (s *Service) Delete(certType, name string) error {
	if !ValidType(certType) {
		return apperr.Validationf("cert type is invalid")
	}
	if !dao.ValidName(name) {
		return apperr.Validationf("cert name is invalid")
	}
	root, _ := s.rootFor(certType)
	if !dao.DomainExists(root, name) {
		return apperr.NotFoundf("cert not found")
	}
	return dao.DeleteDomain(root, name)
}

func validateFileName(certType, file string) error {
	if !dao.ValidName(file) {
		return apperr.Validationf("file name is invalid")
	}
	if certType == apimod.CertTypePublic {
		if file != "priv" && file != "pub" {
			return apperr.Validationf("public certs only allow files priv and pub")
		}
		return nil
	}
	if file != "ca.crt" && file != "server.crt" && file != "server.key" {
		return apperr.Validationf("self-signed certs only allow files ca.crt, server.crt, server.key")
	}
	return nil
}

// validatePEM sanity-checks uploaded content: keys must parse as private
// keys, certificates as x509 certificates. Certificate files may carry a
// chain (leaf first, intermediates after); every block must be a valid
// certificate, trailing garbage is rejected, and each certificate must be
// signed by the next one in the file.
func validatePEM(file, content string) error {
	if file == "priv" || file == "server.key" {
		block, _ := pem.Decode([]byte(content))
		if block == nil {
			return apperr.Validationf("content is not valid PEM")
		}
		return validatePrivateKey(block)
	}
	if file == "pub" || file == "ca.crt" || file == "server.crt" {
		_, err := parseCertificateChain(content)
		return err
	}
	return nil
}

// parseCertificateChain decodes every PEM block in content. The first
// certificate is the leaf; any following certificates must form the
// signing chain in order.
func parseCertificateChain(content string) ([]*x509.Certificate, error) {
	if !strings.HasPrefix(strings.TrimSpace(content), "-----BEGIN") {
		return nil, apperr.Validationf("content is not valid PEM")
	}
	var certs []*x509.Certificate
	rest := []byte(content)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, apperr.Validationf("expected CERTIFICATE block, got %q", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, apperr.Validationf("certificate does not parse: %v", err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, apperr.Validationf("content is not valid PEM")
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		return nil, apperr.Validationf("trailing data after certificate is not valid PEM")
	}
	for i := 0; i+1 < len(certs); i++ {
		if err := certs[i].CheckSignatureFrom(certs[i+1]); err != nil {
			return nil, apperr.Validationf("certificate chain is out of order: certificate %d is not signed by the next certificate", i+1)
		}
	}
	return certs, nil
}

func validatePrivateKey(block *pem.Block) error {
	if _, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return nil
	}
	if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return nil
	}
	if _, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return nil
	}
	return apperr.Validationf("private key does not parse")
}

func parseCertificate(content []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(content)
	if block == nil {
		return nil, fmt.Errorf("not valid PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("certificate parse failed")
	}
	return cert, nil
}

func fillSummary(item *apimod.CertSummary, cert *x509.Certificate) {
	item.CommonName = cert.Subject.CommonName
	item.DNSNames = cert.DNSNames
	item.Issuer = cert.Issuer.CommonName
	item.NotBefore = cert.NotBefore
	item.NotAfter = cert.NotAfter
	item.DaysLeft = int(time.Until(cert.NotAfter).Hours() / 24)
	item.Expired = time.Now().After(cert.NotAfter)
	if item.DNSNames == nil {
		item.DNSNames = []string{}
	}
}

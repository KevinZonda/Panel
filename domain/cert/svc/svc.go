package svc

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
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
// keys, certificates as x509 certificates.
func validatePEM(file, content string) error {
	block, _ := pem.Decode([]byte(content))
	if block == nil {
		return apperr.Validationf("content is not valid PEM")
	}
	if file == "priv" || file == "server.key" {
		return validatePrivateKey(block)
	}
	if file == "pub" || file == "ca.crt" || file == "server.crt" {
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return apperr.Validationf("certificate does not parse: %v", err)
		}
		return nil
	}
	return nil
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

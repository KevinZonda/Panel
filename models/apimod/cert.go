package apimod

import "time"

const (
	CertTypePublic   = "public"
	CertTypeSelfSign = "selfsign"
)

type CertFileInfo struct {
	Name       string    `json:"name"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`
	IsPrivate  bool      `json:"is_private"`
}

type CertSummary struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	CommonName string    `json:"common_name"`
	DNSNames   []string  `json:"dns_names,omitempty"`
	Issuer     string    `json:"issuer"`
	NotBefore  time.Time `json:"not_before"`
	NotAfter   time.Time `json:"not_after"`
	DaysLeft   int       `json:"days_left"`
	Expired    bool      `json:"expired"`
	ParseError string    `json:"parse_error,omitempty"`
}

type CertList struct {
	Items []CertSummary `json:"items"`
}

type CertDetail struct {
	CertSummary
	SerialNumber string         `json:"serial_number"`
	SignatureAlg string         `json:"signature_algorithm"`
	Files        []CertFileInfo `json:"files"`
}

type UploadCertFileRequest struct {
	Content string `json:"content" binding:"required,max=1048576"`
}

type GenerateSelfSignRequest struct {
	Name         string   `json:"name" binding:"required,min=1,max=128"`
	DNSNames     []string `json:"dns_names" binding:"required,min=1,max=32"`
	Days         int      `json:"days" binding:"omitempty,min=1,max=3650"`
	Country      string   `json:"country" binding:"omitempty,max=2"`
	Province     string   `json:"province" binding:"omitempty,max=128"`
	Locality     string   `json:"locality" binding:"omitempty,max=128"`
	Organization string   `json:"organization" binding:"omitempty,max=128"`
	OrgUnit      string   `json:"org_unit" binding:"omitempty,max=128"`
	CACommonName string   `json:"ca_common_name" binding:"omitempty,max=128"`
}

type CertMutationResult struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

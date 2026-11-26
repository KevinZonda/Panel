package apimod

import "time"

type NginxServiceStatus struct {
	Manager     string  `json:"manager"`
	ActiveState *string `json:"active_state"`
	SubState    *string `json:"sub_state"`
}

type NginxConfigTestResult struct {
	OK         bool      `json:"ok"`
	ExitCode   int       `json:"exit_code"`
	Output     string    `json:"output"`
	DurationMS int64     `json:"duration_ms"`
	CheckedAt  time.Time `json:"checked_at"`
}

type NginxStatus struct {
	Installed  bool                  `json:"installed"`
	Running    bool                  `json:"running"`
	PID        *int                  `json:"pid"`
	Version    *string               `json:"version"`
	Service    NginxServiceStatus    `json:"service"`
	ConfigTest NginxConfigTestResult `json:"config_test"`
	CheckedAt  time.Time             `json:"checked_at"`
}

type NginxActionRequest struct {
	Action string `json:"action" binding:"required,oneof=start stop reload restart"`
}

type NginxActionStatus struct {
	Installed bool `json:"installed"`
	Running   bool `json:"running"`
	PID       *int `json:"pid"`
}

type NginxActionResponse struct {
	Action     string            `json:"action"`
	OK         bool              `json:"ok"`
	ExitCode   int               `json:"exit_code"`
	Output     string            `json:"output"`
	DurationMS int64             `json:"duration_ms"`
	FinishedAt time.Time         `json:"finished_at"`
	Status     NginxActionStatus `json:"status"`
}

type NginxSiteSummary struct {
	Name       string    `json:"name"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`
	SHA256     string    `json:"sha256"`
}

type NginxSite struct {
	NginxSiteSummary
	Content string `json:"content"`
}

type NginxSiteList struct {
	Items []NginxSiteSummary `json:"items"`
}

type NginxValidationResult struct {
	OK     bool   `json:"ok"`
	Output string `json:"output"`
}

type CreateNginxSiteRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=128"`
	Content  string `json:"content" binding:"max=1048576"`
	Validate *bool  `json:"validate"`
}

type UpdateNginxSiteRequest struct {
	Content        string `json:"content" binding:"max=1048576"`
	Validate       *bool  `json:"validate"`
	ExpectedSHA256 string `json:"expected_sha256"`
}

type NginxSiteMutationResult struct {
	Name       string                `json:"name"`
	SizeBytes  int64                 `json:"size_bytes"`
	ModifiedAt time.Time             `json:"modified_at"`
	SHA256     string                `json:"sha256"`
	Validation NginxValidationResult `json:"validation"`
}

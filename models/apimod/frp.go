package apimod

import "time"

const (
	FrpTypeServer = "frps"
	FrpTypeClient = "frpc"
)

type FrpServiceStatus struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Description   string `json:"description"`
	ActiveState   string `json:"active_state"`
	SubState      string `json:"sub_state"`
	UnitFileState string `json:"unit_file_state"`
	MainPID       *int   `json:"main_pid"`
	Running       bool   `json:"running"`
	ConfigType    string `json:"config_type,omitempty"`
	ConfigName    string `json:"config_name,omitempty"`
}

type FrpServiceList struct {
	Items []FrpServiceStatus `json:"items"`
}

type FrpServiceActionRequest struct {
	Action string `json:"action" binding:"required,oneof=start stop restart"`
}

type FrpServiceActionResponse struct {
	Service    string           `json:"service"`
	Action     string           `json:"action"`
	OK         bool             `json:"ok"`
	ExitCode   int              `json:"exit_code"`
	Output     string           `json:"output"`
	DurationMS int64            `json:"duration_ms"`
	FinishedAt time.Time        `json:"finished_at"`
	Status     FrpServiceStatus `json:"status"`
}

type FrpConfigSummary struct {
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	ServiceName string    `json:"service_name"`
	SizeBytes   int64     `json:"size_bytes"`
	ModifiedAt  time.Time `json:"modified_at"`
	SHA256      string    `json:"sha256"`
}

type FrpConfigFile struct {
	FrpConfigSummary
	Content string `json:"content"`
}

type FrpConfigList struct {
	Items []FrpConfigSummary `json:"items"`
}

type FrpValidationResult struct {
	OK     bool   `json:"ok"`
	Output string `json:"output"`
}

type CreateFrpConfigRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=128"`
	Content  string `json:"content" binding:"max=1048576"`
	Validate *bool  `json:"validate"`
}

type UpdateFrpConfigRequest struct {
	Content        string `json:"content" binding:"max=1048576"`
	Validate       *bool  `json:"validate"`
	ExpectedSHA256 string `json:"expected_sha256"`
}

type FrpConfigMutationResult struct {
	Type       string              `json:"type"`
	Name       string              `json:"name"`
	SizeBytes  int64               `json:"size_bytes"`
	ModifiedAt time.Time           `json:"modified_at"`
	SHA256     string              `json:"sha256"`
	Validation FrpValidationResult `json:"validation"`
}

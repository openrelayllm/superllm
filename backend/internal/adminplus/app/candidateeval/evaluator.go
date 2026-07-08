package candidateeval

import (
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

const (
	StatusAvailable       = "available"
	StatusUnknown         = "unknown"
	StatusDegraded        = "degraded"
	StatusNeedsProvision  = "needs_provisioning"
	StatusBalanceBlocked  = "balance_blocked"
	StatusBlocked         = "blocked"
	StatusLocalBlocked    = "local_blocked"
	StatusCapacityBlocked = "capacity_blocked"
)

const (
	SourceSupplier       = "supplier"
	SourceSupplierGroup  = "supplier_group"
	SourceSupplierKey    = "supplier_key"
	SourceKeyCapacity    = "key_capacity"
	SourceBalance        = "balance"
	SourceLocalState     = "local_state"
	SourceChannelMonitor = "channel_monitor"
	SourceActiveProbe    = "active_probe"
	SourceModelScope     = "model_scope"
)

const (
	BalanceOK              = "balance_ok"
	BalanceLow             = "balance_low"
	BalanceBlocked         = "balance_blocked"
	BalanceRechargeNeeded  = "recharge_required"
	BalanceUnknown         = "balance_unknown"
	BalanceUnbound         = "unbound"
	KeyCapacityAvailable   = "available"
	KeyCapacityUnknown     = "unknown"
	KeyCapacityLimited     = "limited"
	KeyCapacityExhausted   = "exhausted"
	KeyCapacityUnsupported = "unsupported"
	ModelMatchSupported    = "supported"
	ModelMatchUnknown      = "unknown"
	ModelMatchUnsupported  = "unsupported"
)

type Input struct {
	SupplierID                   int64
	SupplierRuntimeStatus        string
	SupplierHealthStatus         string
	SupplierAccountID            int64
	SupplierAccountRuntimeStatus string
	SupplierAccountHealthStatus  string
	SupplierGroupID              int64
	SupplierGroupStatus          string
	SupplierKeyID                int64
	SupplierKeyStatus            string
	LocalAccountID               int64
	LocalAccountStatus           string
	LocalAccountSchedulable      bool
	LocalAccountTempBlocked      bool
	DriftStatus                  string
	HasUsableBalance             bool
	BalanceStatus                string
	KeyCapacityStatus            string
	ChannelCheckStatus           string
	ChannelRemoteStatus          string
	RequestedModel               string
	SupplierGroupModelFamily     string
	SupplierGroupModelSpec       string
	SupplierGroupName            string
	SupplierGroupProvider        string
	SupplierExternalGroupID      string
	EffectiveRateMultiplier      float64
}

type Evaluation struct {
	CandidateStatus         string  `json:"candidate_status"`
	BlockedReason           string  `json:"blocked_reason,omitempty"`
	CheckSource             string  `json:"check_source,omitempty"`
	BalanceStatus           string  `json:"balance_status"`
	KeyCapacityStatus       string  `json:"key_capacity_status"`
	ModelScope              string  `json:"model_scope,omitempty"`
	ModelMatchStatus        string  `json:"model_match_status,omitempty"`
	EffectiveRateMultiplier float64 `json:"effective_rate_multiplier"`
}

func Evaluate(input Input) Evaluation {
	normalized := normalizeInput(input)
	if normalized.EffectiveRateMultiplier <= 0 {
		return normalized.blocked(StatusBlocked, "rate_missing", SourceSupplierGroup)
	}
	if normalized.SupplierID <= 0 || normalized.SupplierAccountID <= 0 {
		return normalized.blocked(StatusNeedsProvision, "supplier_binding_missing", SourceSupplier)
	}
	if normalized.SupplierRuntimeStatus == "disabled" {
		return normalized.blocked(StatusBlocked, "supplier_disabled", SourceSupplier)
	}
	switch normalized.SupplierHealthStatus {
	case "unavailable", "credential_invalid", "paused":
		return normalized.blocked(StatusBlocked, "supplier_"+normalized.SupplierHealthStatus, SourceSupplier)
	}
	if normalized.SupplierAccountRuntimeStatus == "disabled" {
		return normalized.blocked(StatusBlocked, "supplier_account_disabled", SourceSupplier)
	}
	switch normalized.SupplierAccountHealthStatus {
	case "unavailable", "credential_invalid", "paused":
		return normalized.blocked(StatusBlocked, "supplier_account_"+normalized.SupplierAccountHealthStatus, SourceSupplier)
	}
	if normalized.SupplierGroupID <= 0 {
		return normalized.blocked(StatusNeedsProvision, "supplier_group_missing", SourceSupplierGroup)
	}
	switch normalized.SupplierGroupStatus {
	case "missing", "disabled":
		return normalized.blocked(StatusBlocked, "supplier_group_"+normalized.SupplierGroupStatus, SourceSupplierGroup)
	}
	if normalized.SupplierKeyID <= 0 {
		return normalized.provisioningBlock("supplier_key_missing")
	}
	switch normalized.SupplierKeyStatus {
	case "failed", "disabled":
		return normalized.blocked(StatusBlocked, "supplier_key_"+normalized.SupplierKeyStatus, SourceSupplierKey)
	case "manual_secret_required", "provisioning":
		return normalized.blocked(StatusNeedsProvision, "supplier_key_"+normalized.SupplierKeyStatus, SourceSupplierKey)
	}
	if normalized.LocalAccountID <= 0 {
		return normalized.blocked(StatusNeedsProvision, "local_account_missing", SourceLocalState)
	}
	if normalized.DriftStatus != "" && normalized.DriftStatus != "synced" {
		return normalized.blocked(StatusLocalBlocked, normalized.DriftStatus, SourceLocalState)
	}
	if normalized.LocalAccountStatus != "" && normalized.LocalAccountStatus != "active" {
		return normalized.blocked(StatusLocalBlocked, "local_account_"+normalized.LocalAccountStatus, SourceLocalState)
	}
	if normalized.LocalAccountTempBlocked {
		return normalized.blocked(StatusLocalBlocked, "local_account_temp_unschedulable", SourceLocalState)
	}
	if !normalized.LocalAccountSchedulable {
		return normalized.blocked(StatusLocalBlocked, "local_account_unschedulable", SourceLocalState)
	}
	if normalized.BalanceStatus == BalanceBlocked || normalized.BalanceStatus == BalanceRechargeNeeded {
		return normalized.blocked(StatusBalanceBlocked, BalanceRechargeNeeded, SourceBalance)
	}
	if normalized.BalanceStatus == BalanceUnknown {
		return normalized.blocked(StatusUnknown, BalanceUnknown, SourceBalance)
	}
	if normalized.modelMatchStatus() == ModelMatchUnsupported {
		return normalized.blocked(StatusBlocked, "model_scope_unsupported", SourceModelScope)
	}
	switch normalized.ChannelCheckStatus {
	case "available":
		return normalized.ok(StatusAvailable, SourceChannelMonitor)
	case "slow_first_token", "slow_total":
		return normalized.blocked(StatusDegraded, stringValue(normalized.ChannelCheckStatus), SourceActiveProbe)
	case "remote_unavailable":
		return normalized.blocked(StatusBlocked, "channel_monitor_failed", SourceChannelMonitor)
	case "request_error", "probe_failed":
		return normalized.blocked(StatusBlocked, "channel_active_probe_failed", SourceActiveProbe)
	case "no_local_account":
		return normalized.blocked(StatusNeedsProvision, "no_local_account", SourceLocalState)
	case "untested", "":
		return normalized.blocked(StatusUnknown, "channel_untested", SourceChannelMonitor)
	default:
		return normalized.blocked(StatusUnknown, "channel_unknown", SourceChannelMonitor)
	}
}

func FromLocalAccountOpsRow(row *adminplusdomain.LocalAccountOpsRow) Evaluation {
	return FromLocalAccountOpsRowWithModel(row, "")
}

func FromLocalAccountOpsRowWithModel(row *adminplusdomain.LocalAccountOpsRow, requestedModel string) Evaluation {
	if row == nil {
		return Evaluate(Input{})
	}
	return Evaluate(Input{
		SupplierID:                   row.SupplierID,
		SupplierRuntimeStatus:        row.SupplierRuntimeStatus,
		SupplierHealthStatus:         row.SupplierHealthStatus,
		SupplierAccountID:            row.SupplierAccountID,
		SupplierAccountRuntimeStatus: row.SupplierAccountRuntimeStatus,
		SupplierAccountHealthStatus:  row.SupplierAccountHealthStatus,
		SupplierGroupID:              row.SupplierGroupID,
		SupplierGroupStatus:          row.SupplierGroupStatus,
		SupplierKeyID:                row.SupplierKeyID,
		SupplierKeyStatus:            row.SupplierKeyStatus,
		LocalAccountID:               row.LocalSub2APIAccountID,
		LocalAccountStatus:           row.LocalAccountStatus,
		LocalAccountSchedulable:      row.LocalAccountSchedulable,
		LocalAccountTempBlocked:      row.LocalAccountTempUnschedAt != nil,
		DriftStatus:                  row.DriftStatus,
		HasUsableBalance:             row.HasUsableBalance,
		BalanceStatus:                row.BalanceStatus,
		KeyCapacityStatus:            row.KeyCapacityStatus,
		ChannelCheckStatus:           row.ChannelCheckStatus,
		ChannelRemoteStatus:          row.ChannelRemoteStatus,
		RequestedModel:               requestedModel,
		SupplierGroupModelFamily:     row.SupplierGroupModelFamily,
		SupplierGroupModelSpec:       row.SupplierGroupModelSpec,
		SupplierGroupName:            row.SupplierGroupName,
		SupplierGroupProvider:        row.SupplierGroupProvider,
		SupplierExternalGroupID:      row.SupplierExternalGroupID,
		EffectiveRateMultiplier:      row.EffectiveRateMultiplier,
	})
}

func ApplyToLocalAccountOpsRow(row *adminplusdomain.LocalAccountOpsRow) {
	ApplyToLocalAccountOpsRowForModel(row, "")
}

func ApplyToLocalAccountOpsRowForModel(row *adminplusdomain.LocalAccountOpsRow, requestedModel string) {
	if row == nil {
		return
	}
	evaluation := FromLocalAccountOpsRowWithModel(row, requestedModel)
	row.CandidateStatus = evaluation.CandidateStatus
	row.BlockedReason = evaluation.BlockedReason
	row.CheckSource = evaluation.CheckSource
	row.KeyCapacityStatus = evaluation.KeyCapacityStatus
	row.ModelScope = evaluation.ModelScope
	row.ModelMatchStatus = evaluation.ModelMatchStatus
}

func normalizeInput(input Input) Input {
	input.SupplierRuntimeStatus = normalize(input.SupplierRuntimeStatus)
	input.SupplierHealthStatus = normalize(input.SupplierHealthStatus)
	input.SupplierAccountRuntimeStatus = normalize(input.SupplierAccountRuntimeStatus)
	input.SupplierAccountHealthStatus = normalize(input.SupplierAccountHealthStatus)
	input.SupplierGroupStatus = normalize(input.SupplierGroupStatus)
	input.SupplierKeyStatus = normalize(input.SupplierKeyStatus)
	input.LocalAccountStatus = normalize(input.LocalAccountStatus)
	input.DriftStatus = normalize(input.DriftStatus)
	input.BalanceStatus = normalizeBalanceStatus(input.BalanceStatus, input.HasUsableBalance)
	input.KeyCapacityStatus = normalizeKeyCapacityStatus(input.KeyCapacityStatus)
	input.ChannelCheckStatus = normalize(input.ChannelCheckStatus)
	input.ChannelRemoteStatus = normalize(input.ChannelRemoteStatus)
	input.RequestedModel = normalizeModelText(input.RequestedModel)
	input.SupplierGroupModelFamily = strings.TrimSpace(input.SupplierGroupModelFamily)
	input.SupplierGroupModelSpec = strings.TrimSpace(input.SupplierGroupModelSpec)
	input.SupplierGroupName = strings.TrimSpace(input.SupplierGroupName)
	input.SupplierGroupProvider = strings.TrimSpace(input.SupplierGroupProvider)
	input.SupplierExternalGroupID = strings.TrimSpace(input.SupplierExternalGroupID)
	return input
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeBalanceStatus(value string, hasUsableBalance bool) string {
	value = normalize(value)
	if hasUsableBalance || value == "usable" || value == "ok" || value == BalanceOK {
		return BalanceOK
	}
	switch value {
	case "insufficient", "blocked", BalanceBlocked, BalanceRechargeNeeded:
		return BalanceRechargeNeeded
	case "low", BalanceLow:
		return BalanceLow
	case "unbound":
		return BalanceUnbound
	case "", "unknown":
		return BalanceUnknown
	default:
		return value
	}
}

func normalizeKeyCapacityStatus(value string) string {
	switch normalize(value) {
	case "", "unknown":
		return KeyCapacityUnknown
	case "available", "ok", "unlimited":
		return KeyCapacityAvailable
	case "limited", "low":
		return KeyCapacityLimited
	case "exhausted", "full", "blocked":
		return KeyCapacityExhausted
	case "unsupported":
		return KeyCapacityUnsupported
	default:
		return normalize(value)
	}
}

func (input Input) provisioningBlock(reason string) Evaluation {
	if input.KeyCapacityStatus == KeyCapacityExhausted {
		return input.blocked(StatusCapacityBlocked, "key_capacity_exhausted", SourceKeyCapacity)
	}
	return input.blocked(StatusNeedsProvision, reason, SourceSupplierKey)
}

func (input Input) ok(status string, source string) Evaluation {
	return Evaluation{
		CandidateStatus:         status,
		CheckSource:             source,
		BalanceStatus:           input.BalanceStatus,
		KeyCapacityStatus:       input.KeyCapacityStatus,
		ModelScope:              input.modelScope(),
		ModelMatchStatus:        input.modelMatchStatus(),
		EffectiveRateMultiplier: input.EffectiveRateMultiplier,
	}
}

func (input Input) blocked(status string, reason string, source string) Evaluation {
	result := input.ok(status, source)
	result.BlockedReason = reason
	return result
}

func stringValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func (input Input) modelScope() string {
	parts := make([]string, 0, 2)
	if value := strings.TrimSpace(input.SupplierGroupModelFamily); value != "" {
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(input.SupplierGroupModelSpec); value != "" && !sameFold(value, input.SupplierGroupModelFamily) {
		parts = append(parts, value)
	}
	return strings.Join(parts, " / ")
}

func (input Input) modelMatchStatus() string {
	if strings.TrimSpace(input.RequestedModel) == "" {
		return ""
	}
	requestedFamily := canonicalModelFamily(input.RequestedModel)
	scopeFamilies := make([]string, 0, 5)
	for _, value := range []string{
		input.SupplierGroupModelFamily,
		input.SupplierGroupModelSpec,
		input.SupplierGroupName,
		input.SupplierExternalGroupID,
		input.SupplierGroupProvider,
	} {
		if family := canonicalModelFamily(value); family != "" {
			scopeFamilies = append(scopeFamilies, family)
		}
	}
	if requestedFamily == "" || len(scopeFamilies) == 0 {
		return ModelMatchUnknown
	}
	for _, family := range scopeFamilies {
		if family == requestedFamily {
			return ModelMatchSupported
		}
	}
	return ModelMatchUnsupported
}

func canonicalModelFamily(value string) string {
	value = normalizeModelText(value)
	switch {
	case strings.Contains(value, "claude"), strings.Contains(value, "anthropic"):
		return "claude"
	case strings.Contains(value, "gemini"), strings.Contains(value, "google"):
		return "gemini"
	case strings.Contains(value, "grok"), strings.Contains(value, "xai"):
		return "grok"
	case strings.Contains(value, "antigravity"):
		return "antigravity"
	case strings.Contains(value, "openai"), strings.Contains(value, "gpt"), strings.Contains(value, "codex"):
		return "openai"
	default:
		return ""
	}
}

func normalizeModelText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("_", "-", "/", "-", "\\", "-", ".", "-").Replace(value)
	return strings.Join(strings.Fields(value), "-")
}

func sameFold(left string, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

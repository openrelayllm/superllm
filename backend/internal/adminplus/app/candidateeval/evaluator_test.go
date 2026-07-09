package candidateeval

import (
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestEvaluatePrioritizesBalanceBeforeChannelFailure(t *testing.T) {
	result := Evaluate(baseInput(Input{
		HasUsableBalance:   false,
		BalanceStatus:      "insufficient",
		ChannelCheckStatus: "remote_unavailable",
	}))

	require.Equal(t, StatusBalanceBlocked, result.CandidateStatus)
	require.Equal(t, BalanceRechargeNeeded, result.BlockedReason)
	require.Equal(t, SourceBalance, result.CheckSource)
	require.Equal(t, BalanceRechargeNeeded, result.BalanceStatus)
}

func TestEvaluateClassifiesChannelMonitorFailure(t *testing.T) {
	result := Evaluate(baseInput(Input{
		ChannelCheckStatus: "remote_unavailable",
	}))

	require.Equal(t, StatusBlocked, result.CandidateStatus)
	require.Equal(t, "channel_monitor_failed", result.BlockedReason)
	require.Equal(t, SourceChannelMonitor, result.CheckSource)
}

func TestEvaluateClassifiesUntestedCandidateAsUnknown(t *testing.T) {
	result := Evaluate(baseInput(Input{
		ChannelCheckStatus: "untested",
	}))

	require.Equal(t, StatusUnknown, result.CandidateStatus)
	require.Equal(t, "channel_untested", result.BlockedReason)
	require.Equal(t, SourceChannelMonitor, result.CheckSource)
}

func TestEvaluateClassifiesHealthyCandidateAsAvailable(t *testing.T) {
	result := Evaluate(baseInput(Input{
		ChannelCheckStatus: "available",
	}))

	require.Equal(t, StatusAvailable, result.CandidateStatus)
	require.Empty(t, result.BlockedReason)
	require.Equal(t, SourceChannelMonitor, result.CheckSource)
	require.Equal(t, BalanceOK, result.BalanceStatus)
	require.Equal(t, KeyCapacityUnknown, result.KeyCapacityStatus)
}

func TestEvaluateBlocksMissingKeyWhenCapacityExhausted(t *testing.T) {
	input := baseInput(Input{KeyCapacityStatus: "exhausted"})
	input.SupplierKeyID = 0

	result := Evaluate(input)

	require.Equal(t, StatusCapacityBlocked, result.CandidateStatus)
	require.Equal(t, "key_capacity_exhausted", result.BlockedReason)
	require.Equal(t, SourceKeyCapacity, result.CheckSource)
}

func TestEvaluateSupportsMatchingModelScope(t *testing.T) {
	result := Evaluate(baseInput(Input{
		RequestedModel:           "gpt-4o-mini",
		SupplierGroupModelFamily: "OpenAI",
		SupplierGroupModelSpec:   "GPT-4o",
	}))

	require.Equal(t, StatusAvailable, result.CandidateStatus)
	require.Empty(t, result.BlockedReason)
	require.Equal(t, SourceChannelMonitor, result.CheckSource)
	require.Equal(t, "OpenAI / GPT-4o", result.ModelScope)
	require.Equal(t, ModelMatchSupported, result.ModelMatchStatus)
}

func TestEvaluateKeepsUnknownModelScopeAvailable(t *testing.T) {
	result := Evaluate(baseInput(Input{RequestedModel: "gpt-4o-mini"}))

	require.Equal(t, StatusAvailable, result.CandidateStatus)
	require.Empty(t, result.BlockedReason)
	require.Equal(t, ModelMatchUnknown, result.ModelMatchStatus)
}

func TestEvaluateBlocksExplicitlyUnsupportedModelScope(t *testing.T) {
	result := Evaluate(baseInput(Input{
		RequestedModel:           "claude-3-5-sonnet",
		SupplierGroupModelFamily: "OpenAI",
		SupplierGroupModelSpec:   "GPT-4o",
	}))

	require.Equal(t, StatusBlocked, result.CandidateStatus)
	require.Equal(t, "model_scope_unsupported", result.BlockedReason)
	require.Equal(t, SourceModelScope, result.CheckSource)
	require.Equal(t, ModelMatchUnsupported, result.ModelMatchStatus)
}

func TestEvaluateBlocksExplicitPurityFailure(t *testing.T) {
	result := Evaluate(baseInput(Input{
		PurityVerdict: "invalid_or_unavailable",
	}))

	require.Equal(t, StatusBlocked, result.CandidateStatus)
	require.Equal(t, "purity_failed", result.BlockedReason)
	require.Equal(t, SourcePurity, result.CheckSource)
	require.Equal(t, PurityFail, result.PurityStatus)
}

func TestEvaluateDegradesPurityRiskWhenChannelIsAvailable(t *testing.T) {
	result := Evaluate(baseInput(Input{
		PurityVerdict: "partial_compatible",
	}))

	require.Equal(t, StatusDegraded, result.CandidateStatus)
	require.Equal(t, "purity_risk", result.BlockedReason)
	require.Equal(t, SourcePurity, result.CheckSource)
	require.Equal(t, PurityWarn, result.PurityStatus)
}

func TestEvaluateKeepsUnknownPurityAvailable(t *testing.T) {
	result := Evaluate(baseInput(Input{
		PurityStatus: "unknown",
	}))

	require.Equal(t, StatusAvailable, result.CandidateStatus)
	require.Empty(t, result.BlockedReason)
	require.Equal(t, SourceChannelMonitor, result.CheckSource)
	require.Equal(t, PurityUnknown, result.PurityStatus)
}

func TestEvaluateBlocksUnavailableBoundProxy(t *testing.T) {
	result := Evaluate(baseInput(Input{
		LocalAccountProxyID:     9,
		LocalAccountProxyStatus: "expired",
	}))

	require.Equal(t, StatusBlocked, result.CandidateStatus)
	require.Equal(t, "proxy_expired", result.BlockedReason)
	require.Equal(t, SourceProxy, result.CheckSource)
}

func TestEvaluateKeepsUnboundOrUnknownProxyAvailable(t *testing.T) {
	unbound := Evaluate(baseInput(Input{}))
	require.Equal(t, StatusAvailable, unbound.CandidateStatus)
	require.Empty(t, unbound.BlockedReason)

	unknown := Evaluate(baseInput(Input{
		LocalAccountProxyID:     9,
		LocalAccountProxyStatus: "unknown",
	}))
	require.Equal(t, StatusAvailable, unknown.CandidateStatus)
	require.Empty(t, unknown.BlockedReason)
	require.Equal(t, SourceChannelMonitor, unknown.CheckSource)
}

func TestEvaluateDoesNotHideChannelFailureWithPurityWarning(t *testing.T) {
	result := Evaluate(baseInput(Input{
		ChannelCheckStatus: "remote_unavailable",
		PurityVerdict:      "partial_compatible",
	}))

	require.Equal(t, StatusBlocked, result.CandidateStatus)
	require.Equal(t, "channel_monitor_failed", result.BlockedReason)
	require.Equal(t, SourceChannelMonitor, result.CheckSource)
	require.Equal(t, PurityWarn, result.PurityStatus)
}

func TestApplyToLocalAccountOpsRow(t *testing.T) {
	row := &adminplusdomain.LocalAccountOpsRow{
		SupplierID:                   7,
		SupplierRuntimeStatus:        "active",
		SupplierAccountID:            77,
		SupplierAccountRuntimeStatus: "active",
		SupplierGroupID:              1001,
		SupplierGroupStatus:          "active",
		SupplierKeyID:                9001,
		SupplierKeyStatus:            "bound",
		LocalSub2APIAccountID:        42,
		LocalAccountStatus:           "active",
		LocalAccountSchedulable:      true,
		DriftStatus:                  "synced",
		HasUsableBalance:             true,
		BalanceStatus:                "usable",
		ChannelCheckStatus:           "available",
		EffectiveRateMultiplier:      0.2,
	}

	ApplyToLocalAccountOpsRow(row)

	require.Equal(t, StatusAvailable, row.CandidateStatus)
	require.Empty(t, row.BlockedReason)
	require.Equal(t, SourceChannelMonitor, row.CheckSource)
	require.Equal(t, KeyCapacityUnknown, row.KeyCapacityStatus)
}

func TestApplyToLocalAccountOpsRowForModel(t *testing.T) {
	row := &adminplusdomain.LocalAccountOpsRow{
		SupplierID:                   7,
		SupplierRuntimeStatus:        "active",
		SupplierAccountID:            77,
		SupplierAccountRuntimeStatus: "active",
		SupplierGroupID:              1001,
		SupplierGroupStatus:          "active",
		SupplierGroupModelFamily:     "Claude",
		SupplierGroupModelSpec:       "Sonnet",
		SupplierKeyID:                9001,
		SupplierKeyStatus:            "bound",
		LocalSub2APIAccountID:        42,
		LocalAccountStatus:           "active",
		LocalAccountSchedulable:      true,
		DriftStatus:                  "synced",
		HasUsableBalance:             true,
		BalanceStatus:                "usable",
		ChannelCheckStatus:           "available",
		EffectiveRateMultiplier:      0.2,
	}

	ApplyToLocalAccountOpsRowForModel(row, "gpt-4o")

	require.Equal(t, StatusBlocked, row.CandidateStatus)
	require.Equal(t, "model_scope_unsupported", row.BlockedReason)
	require.Equal(t, SourceModelScope, row.CheckSource)
	require.Equal(t, "Claude / Sonnet", row.ModelScope)
	require.Equal(t, ModelMatchUnsupported, row.ModelMatchStatus)
}

func baseInput(overrides Input) Input {
	input := Input{
		SupplierID:                   7,
		SupplierRuntimeStatus:        "active",
		SupplierAccountID:            77,
		SupplierAccountRuntimeStatus: "active",
		SupplierGroupID:              1001,
		SupplierGroupStatus:          "active",
		SupplierKeyID:                9001,
		SupplierKeyStatus:            "bound",
		LocalAccountID:               42,
		LocalAccountStatus:           "active",
		LocalAccountSchedulable:      true,
		DriftStatus:                  "synced",
		HasUsableBalance:             true,
		BalanceStatus:                "usable",
		ChannelCheckStatus:           "available",
		EffectiveRateMultiplier:      0.2,
	}
	if overrides.SupplierID != 0 {
		input.SupplierID = overrides.SupplierID
	}
	if overrides.SupplierRuntimeStatus != "" {
		input.SupplierRuntimeStatus = overrides.SupplierRuntimeStatus
	}
	if overrides.SupplierHealthStatus != "" {
		input.SupplierHealthStatus = overrides.SupplierHealthStatus
	}
	if overrides.SupplierAccountID != 0 {
		input.SupplierAccountID = overrides.SupplierAccountID
	}
	if overrides.SupplierAccountRuntimeStatus != "" {
		input.SupplierAccountRuntimeStatus = overrides.SupplierAccountRuntimeStatus
	}
	if overrides.SupplierAccountHealthStatus != "" {
		input.SupplierAccountHealthStatus = overrides.SupplierAccountHealthStatus
	}
	if overrides.SupplierGroupID != 0 {
		input.SupplierGroupID = overrides.SupplierGroupID
	}
	if overrides.SupplierGroupStatus != "" {
		input.SupplierGroupStatus = overrides.SupplierGroupStatus
	}
	if overrides.SupplierKeyID != 0 {
		input.SupplierKeyID = overrides.SupplierKeyID
	}
	if overrides.SupplierKeyStatus != "" {
		input.SupplierKeyStatus = overrides.SupplierKeyStatus
	}
	if overrides.LocalAccountID != 0 {
		input.LocalAccountID = overrides.LocalAccountID
	}
	if overrides.LocalAccountStatus != "" {
		input.LocalAccountStatus = overrides.LocalAccountStatus
	}
	if overrides.LocalAccountSchedulable {
		input.LocalAccountSchedulable = true
	}
	if overrides.LocalAccountTempBlocked {
		input.LocalAccountTempBlocked = true
	}
	if overrides.LocalAccountProxyID != 0 {
		input.LocalAccountProxyID = overrides.LocalAccountProxyID
	}
	if overrides.LocalAccountProxyStatus != "" {
		input.LocalAccountProxyStatus = overrides.LocalAccountProxyStatus
	}
	if overrides.DriftStatus != "" {
		input.DriftStatus = overrides.DriftStatus
	}
	if overrides.HasUsableBalance {
		input.HasUsableBalance = true
	}
	if overrides.BalanceStatus != "" {
		input.BalanceStatus = overrides.BalanceStatus
		if overrides.BalanceStatus == "insufficient" {
			input.HasUsableBalance = false
		}
	}
	if overrides.KeyCapacityStatus != "" {
		input.KeyCapacityStatus = overrides.KeyCapacityStatus
	}
	if overrides.ChannelCheckStatus != "" {
		input.ChannelCheckStatus = overrides.ChannelCheckStatus
	}
	if overrides.ChannelRemoteStatus != "" {
		input.ChannelRemoteStatus = overrides.ChannelRemoteStatus
	}
	if overrides.RequestedModel != "" {
		input.RequestedModel = overrides.RequestedModel
	}
	if overrides.SupplierGroupModelFamily != "" {
		input.SupplierGroupModelFamily = overrides.SupplierGroupModelFamily
	}
	if overrides.SupplierGroupModelSpec != "" {
		input.SupplierGroupModelSpec = overrides.SupplierGroupModelSpec
	}
	if overrides.SupplierGroupName != "" {
		input.SupplierGroupName = overrides.SupplierGroupName
	}
	if overrides.SupplierGroupProvider != "" {
		input.SupplierGroupProvider = overrides.SupplierGroupProvider
	}
	if overrides.SupplierExternalGroupID != "" {
		input.SupplierExternalGroupID = overrides.SupplierExternalGroupID
	}
	if overrides.PurityStatus != "" {
		input.PurityStatus = overrides.PurityStatus
	}
	if overrides.PurityVerdict != "" {
		input.PurityVerdict = overrides.PurityVerdict
	}
	if overrides.PurityModelIdentityStatus != "" {
		input.PurityModelIdentityStatus = overrides.PurityModelIdentityStatus
	}
	if overrides.PurityTokenAuditStatus != "" {
		input.PurityTokenAuditStatus = overrides.PurityTokenAuditStatus
	}
	if overrides.PurityScore != 0 {
		input.PurityScore = overrides.PurityScore
	}
	if overrides.EffectiveRateMultiplier != 0 {
		input.EffectiveRateMultiplier = overrides.EffectiveRateMultiplier
	}
	return input
}

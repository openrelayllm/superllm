package sub2api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	remoteRoutingAdminBaseURLEnv = "ADMIN_PLUS_SUB2API_ADMIN_BASE_URL"
	remoteRoutingAdminAPIKeyEnv  = "ADMIN_PLUS_SUB2API_ADMIN_API_KEY"
)

type RemoteAdminAPIRoutingPort struct {
	baseURL string
	apiKey  string
	client  *http.Client
	local   *SQLRepository
}

func NewRemoteAdminAPIRoutingPort(baseURL string, apiKey string, client *http.Client, local *SQLRepository) (*RemoteAdminAPIRoutingPort, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" || apiKey == "" {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_REMOTE_ROUTING_CONFIG_REQUIRED", "sub2api remote routing base url and admin api key are required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_REMOTE_ROUTING_BASE_URL_INVALID", "sub2api remote routing base url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_REMOTE_ROUTING_BASE_URL_INVALID", "sub2api remote routing base url must use http or https")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &RemoteAdminAPIRoutingPort{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  client,
		local:   local,
	}, nil
}

func NewRemoteAdminAPIRoutingPortFromConfig(cfg *config.Config, client *http.Client, local *SQLRepository) (*RemoteAdminAPIRoutingPort, error) {
	if cfg == nil {
		return NewRemoteAdminAPIRoutingPort(os.Getenv(remoteRoutingAdminBaseURLEnv), os.Getenv(remoteRoutingAdminAPIKeyEnv), client, local)
	}
	return NewRemoteAdminAPIRoutingPort(cfg.AdminPlus.Sub2APIAdminBaseURL, cfg.AdminPlus.Sub2APIAdminAPIKey, client, local)
}

func ShouldUseRemoteAdminAPIRoutingPortFromConfig(cfg *config.Config) bool {
	if cfg == nil {
		return strings.TrimSpace(os.Getenv(remoteRoutingAdminBaseURLEnv)) != "" &&
			strings.TrimSpace(os.Getenv(remoteRoutingAdminAPIKeyEnv)) != ""
	}
	return strings.TrimSpace(cfg.AdminPlus.Sub2APIAdminBaseURL) != "" &&
		strings.TrimSpace(cfg.AdminPlus.Sub2APIAdminAPIKey) != ""
}

func (p *RemoteAdminAPIRoutingPort) GetGroupAvailability(ctx context.Context, groupID int64, platform string) (*RoutingGroupAvailability, error) {
	if p == nil || p.local == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_REMOTE_ROUTING_READ_REPOSITORY_REQUIRED", "sub2api remote routing requires a local read repository")
	}
	return p.local.GetGroupAvailability(ctx, groupID, platform)
}

func (p *RemoteAdminAPIRoutingPort) GetAccount(ctx context.Context, accountID int64) (*Sub2APIAccountSnapshot, error) {
	if accountID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_ACCOUNT_ID_INVALID", "invalid account id")
	}
	var account remoteSub2APIAccountDTO
	if err := p.doJSON(ctx, http.MethodGet, "/api/v1/admin/accounts/"+strconv.FormatInt(accountID, 10), nil, &account); err != nil {
		return nil, err
	}
	return account.toSnapshot(), nil
}

func (p *RemoteAdminAPIRoutingPort) EnsureAccountInGroup(ctx context.Context, accountID int64, groupID int64) (*Sub2APIAccountSnapshot, error) {
	if accountID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_ACCOUNT_ID_INVALID", "invalid account id")
	}
	if groupID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	account, err := p.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if containsInt64(account.GroupIDs, groupID) {
		return account, nil
	}
	groupIDs := append([]int64(nil), account.GroupIDs...)
	groupIDs = append(groupIDs, groupID)
	groupIDs = uniqueSortedPositiveInt64s(groupIDs)
	return p.updateAccountGroups(ctx, accountID, groupIDs)
}

func (p *RemoteAdminAPIRoutingPort) SetAccountSchedulable(ctx context.Context, accountID int64, schedulable bool, reason string) (*Sub2APIAccountSnapshot, error) {
	if accountID <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_ACCOUNT_ID_INVALID", "invalid account id")
	}
	payload := map[string]any{
		"schedulable": schedulable,
	}
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		payload["reason"] = trimmed
	}
	var account remoteSub2APIAccountDTO
	if err := p.doJSON(ctx, http.MethodPost, "/api/v1/admin/accounts/"+strconv.FormatInt(accountID, 10)+"/schedulable", payload, &account); err != nil {
		return nil, err
	}
	return account.toSnapshot(), nil
}

func (p *RemoteAdminAPIRoutingPort) PreviewLocalAccountOpsAction(ctx context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	if p == nil || p.local == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_REMOTE_ROUTING_READ_REPOSITORY_REQUIRED", "sub2api remote routing requires a local read repository")
	}
	return p.local.PreviewLocalAccountOpsAction(ctx, input)
}

func (p *RemoteAdminAPIRoutingPort) ApplyLocalAccountOpsAction(ctx context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	if input.DryRun {
		return p.PreviewLocalAccountOpsAction(ctx, input)
	}
	plan, err := p.prepareLocalAccountOpsAction(ctx, input)
	if err != nil {
		return nil, err
	}
	if plan.Blocked && !input.AllowEmptyPool {
		return plan, nil
	}
	plan.Blocked = false
	plan.BlockedReason = ""

	switch input.Action {
	case adminplusdomain.LocalAccountOpsActionSetSchedulable:
		if input.Schedulable == nil {
			return nil, infraerrors.New(http.StatusBadRequest, "LOCAL_ACCOUNT_OPS_SCHEDULABLE_REQUIRED", "schedulable is required")
		}
		updated, err := p.applyRemoteSchedulable(ctx, input.AccountIDs, *input.Schedulable, input.Reason)
		if err != nil {
			return nil, err
		}
		plan.UpdatedAccounts = updated
	case adminplusdomain.LocalAccountOpsActionAddToGroups:
		added, err := p.applyRemoteGroupAdd(ctx, input.AccountIDs, input.GroupIDs)
		if err != nil {
			return nil, err
		}
		plan.AddedBindings = added
	case adminplusdomain.LocalAccountOpsActionRemoveFromGroups:
		removed, err := p.applyRemoteGroupRemove(ctx, input.AccountIDs, input.GroupIDs)
		if err != nil {
			return nil, err
		}
		plan.RemovedBindings = removed
	default:
		return nil, infraerrors.New(http.StatusBadRequest, "LOCAL_ACCOUNT_OPS_ACTION_INVALID", "invalid local account operation")
	}
	p.acceptRemoteLocalAccountState(ctx, input, plan)
	return plan, nil
}

func (p *RemoteAdminAPIRoutingPort) ResolveLocalAccountState(ctx context.Context, input LocalAccountStateResolutionInput) (*adminplusdomain.LocalAccountStateResolutionResult, error) {
	if p == nil || p.local == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_REMOTE_ROUTING_READ_REPOSITORY_REQUIRED", "sub2api remote routing requires a local read repository")
	}
	return p.local.ResolveLocalAccountState(ctx, input)
}

func (p *RemoteAdminAPIRoutingPort) TryRoutingRefillLock(ctx context.Context, groupID int64) (RoutingRefillUnlockFunc, bool, error) {
	if p != nil && p.local != nil {
		return p.local.TryRoutingRefillLock(ctx, groupID)
	}
	return func() error { return nil }, true, nil
}

func (p *RemoteAdminAPIRoutingPort) prepareLocalAccountOpsAction(ctx context.Context, input LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	if p == nil || p.local == nil || p.local.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "SUB2API_REMOTE_ROUTING_READ_REPOSITORY_REQUIRED", "sub2api remote routing requires a local read repository")
	}
	tx, err := p.local.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := p.local.syncLocalAccountState(ctx, tx, LocalAccountStateSyncInput{AccountIDs: input.AccountIDs, Limit: len(input.AccountIDs)}, false); err != nil {
		return nil, err
	}
	if pending, err := listPendingLocalAccountStateDrift(ctx, tx, input.AccountIDs); err != nil {
		return nil, err
	} else if len(pending) > 0 {
		return &adminplusdomain.LocalAccountOpsActionResult{
			Action:        input.Action,
			AccountIDs:    append([]int64(nil), input.AccountIDs...),
			GroupIDs:      append([]int64(nil), input.GroupIDs...),
			Blocked:       true,
			BlockedReason: "LOCAL_ACCOUNT_STATE_DRIFT_PENDING",
			Warnings:      []string{"检测到 Sub2API 原后台手工改动，请先同步并采纳或恢复本地状态"},
		}, nil
	}

	plan, err := p.local.buildLocalAccountOpsActionPlan(ctx, tx, input, false)
	if err != nil {
		return nil, err
	}
	if plan.Blocked && !input.AllowEmptyPool {
		return plan, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	plan.DryRun = false
	return plan, nil
}

func (p *RemoteAdminAPIRoutingPort) acceptRemoteLocalAccountState(ctx context.Context, input LocalAccountOpsActionInput, result *adminplusdomain.LocalAccountOpsActionResult) {
	if p == nil || p.local == nil || p.local.db == nil || result == nil {
		return
	}
	tx, err := p.local.db.BeginTx(ctx, nil)
	if err != nil {
		result.Warnings = append(result.Warnings, "远程写回已完成，但本地状态基线同步失败")
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := p.local.syncLocalAccountState(ctx, tx, LocalAccountStateSyncInput{AccountIDs: input.AccountIDs, Limit: len(input.AccountIDs)}, false); err != nil {
		result.Warnings = append(result.Warnings, "远程写回已完成，但本地状态基线同步失败")
		return
	}
	if err := acceptLocalAccountStateSnapshots(ctx, tx, input.AccountIDs); err != nil {
		result.Warnings = append(result.Warnings, "远程写回已完成，但本地状态基线采纳失败")
		return
	}
	if err := tx.Commit(); err != nil {
		result.Warnings = append(result.Warnings, "远程写回已完成，但本地状态基线提交失败")
	}
}

func (p *RemoteAdminAPIRoutingPort) applyRemoteSchedulable(ctx context.Context, accountIDs []int64, schedulable bool, reason string) (int64, error) {
	var updated int64
	for _, accountID := range accountIDs {
		before, err := p.GetAccount(ctx, accountID)
		if err != nil {
			return updated, err
		}
		after, err := p.SetAccountSchedulable(ctx, accountID, schedulable, reason)
		if err != nil {
			return updated, err
		}
		if before.Schedulable != after.Schedulable {
			updated++
		}
	}
	return updated, nil
}

func (p *RemoteAdminAPIRoutingPort) applyRemoteGroupAdd(ctx context.Context, accountIDs []int64, groupIDs []int64) (int64, error) {
	var added int64
	for _, accountID := range accountIDs {
		account, err := p.GetAccount(ctx, accountID)
		if err != nil {
			return added, err
		}
		next := append([]int64(nil), account.GroupIDs...)
		for _, groupID := range groupIDs {
			if groupID <= 0 || containsInt64(next, groupID) {
				continue
			}
			next = append(next, groupID)
			added++
		}
		if len(next) == len(account.GroupIDs) {
			continue
		}
		if _, err := p.updateAccountGroups(ctx, accountID, uniqueSortedPositiveInt64s(next)); err != nil {
			return added, err
		}
	}
	return added, nil
}

func (p *RemoteAdminAPIRoutingPort) applyRemoteGroupRemove(ctx context.Context, accountIDs []int64, groupIDs []int64) (int64, error) {
	removeSet := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID > 0 {
			removeSet[groupID] = struct{}{}
		}
	}
	var removed int64
	for _, accountID := range accountIDs {
		account, err := p.GetAccount(ctx, accountID)
		if err != nil {
			return removed, err
		}
		next := make([]int64, 0, len(account.GroupIDs))
		for _, groupID := range account.GroupIDs {
			if _, ok := removeSet[groupID]; ok {
				removed++
				continue
			}
			next = append(next, groupID)
		}
		if len(next) == len(account.GroupIDs) {
			continue
		}
		if _, err := p.updateAccountGroups(ctx, accountID, uniqueSortedPositiveInt64s(next)); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

func (p *RemoteAdminAPIRoutingPort) updateAccountGroups(ctx context.Context, accountID int64, groupIDs []int64) (*Sub2APIAccountSnapshot, error) {
	payload := map[string]any{
		"group_ids":                  groupIDs,
		"confirm_mixed_channel_risk": true,
	}
	var account remoteSub2APIAccountDTO
	if err := p.doJSON(ctx, http.MethodPut, "/api/v1/admin/accounts/"+strconv.FormatInt(accountID, 10), payload, &account); err != nil {
		return nil, err
	}
	return account.toSnapshot(), nil
}

func (p *RemoteAdminAPIRoutingPort) doJSON(ctx context.Context, method string, path string, payload any, out any) error {
	if p == nil || p.client == nil {
		return infraerrors.New(http.StatusInternalServerError, "SUB2API_REMOTE_ROUTING_NOT_CONFIGURED", "sub2api remote routing is not configured")
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return infraerrors.New(http.StatusBadGateway, "SUB2API_REMOTE_ROUTING_REQUEST_FAILED", "failed to request sub2api admin api").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return remoteSub2APIHTTPError(resp.StatusCode, data)
	}
	if out == nil {
		return nil
	}
	if err := decodeRemoteSub2APIResponse(data, out); err != nil {
		return infraerrors.New(http.StatusBadGateway, "SUB2API_REMOTE_ROUTING_RESPONSE_INVALID", "sub2api admin api response is invalid").WithCause(err)
	}
	return nil
}

type remoteSub2APIEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Reason  string          `json:"reason"`
	Data    json.RawMessage `json:"data"`
}

type remoteSub2APIAccountDTO struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Platform    string  `json:"platform"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	Schedulable bool    `json:"schedulable"`
	GroupIDs    []int64 `json:"group_ids"`
}

func (a remoteSub2APIAccountDTO) toSnapshot() *Sub2APIAccountSnapshot {
	return &Sub2APIAccountSnapshot{
		AccountID:   a.ID,
		Name:        a.Name,
		Platform:    a.Platform,
		Type:        a.Type,
		Status:      a.Status,
		Schedulable: a.Schedulable,
		GroupIDs:    append([]int64(nil), a.GroupIDs...),
	}
}

func decodeRemoteSub2APIResponse(data []byte, out any) error {
	var envelope remoteSub2APIEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Data != nil {
		if envelope.Code != 0 {
			return fmt.Errorf("sub2api returned code %d: %s", envelope.Code, envelope.Message)
		}
		return json.Unmarshal(envelope.Data, out)
	}
	return json.Unmarshal(data, out)
}

func remoteSub2APIHTTPError(statusCode int, data []byte) error {
	var envelope remoteSub2APIEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil {
		reason := strings.TrimSpace(envelope.Reason)
		if reason == "" {
			reason = "SUB2API_REMOTE_ROUTING_BAD_STATUS"
		}
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = http.StatusText(statusCode)
		}
		return infraerrors.New(statusCode, reason, message)
	}
	message := strings.TrimSpace(string(data))
	if message == "" {
		message = http.StatusText(statusCode)
	}
	return infraerrors.New(statusCode, "SUB2API_REMOTE_ROUTING_BAD_STATUS", message)
}

func uniqueSortedPositiveInt64s(items []int64) []int64 {
	seen := make(map[int64]struct{}, len(items))
	out := make([]int64, 0, len(items))
	for _, item := range items {
		if item <= 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

type FailingRoutingPort struct {
	err error
}

func NewFailingRoutingPort(err error) *FailingRoutingPort {
	if err == nil {
		err = infraerrors.New(http.StatusInternalServerError, "SUB2API_REMOTE_ROUTING_NOT_CONFIGURED", "sub2api remote routing is not configured")
	}
	return &FailingRoutingPort{err: err}
}

func (p *FailingRoutingPort) GetGroupAvailability(context.Context, int64, string) (*RoutingGroupAvailability, error) {
	return nil, p.err
}

func (p *FailingRoutingPort) GetAccount(context.Context, int64) (*Sub2APIAccountSnapshot, error) {
	return nil, p.err
}

func (p *FailingRoutingPort) EnsureAccountInGroup(context.Context, int64, int64) (*Sub2APIAccountSnapshot, error) {
	return nil, p.err
}

func (p *FailingRoutingPort) SetAccountSchedulable(context.Context, int64, bool, string) (*Sub2APIAccountSnapshot, error) {
	return nil, p.err
}

func (p *FailingRoutingPort) PreviewLocalAccountOpsAction(context.Context, LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	return nil, p.err
}

func (p *FailingRoutingPort) ApplyLocalAccountOpsAction(context.Context, LocalAccountOpsActionInput) (*adminplusdomain.LocalAccountOpsActionResult, error) {
	return nil, p.err
}

func (p *FailingRoutingPort) ResolveLocalAccountState(context.Context, LocalAccountStateResolutionInput) (*adminplusdomain.LocalAccountStateResolutionResult, error) {
	return nil, p.err
}

var _ Sub2APIRoutingPort = (*RemoteAdminAPIRoutingPort)(nil)
var _ RoutingRefillLocker = (*RemoteAdminAPIRoutingPort)(nil)
var _ Sub2APIRoutingPort = (*FailingRoutingPort)(nil)

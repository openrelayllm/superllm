package proxy

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SecretCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type Repository interface {
	CenterStatus(ctx context.Context) (*adminplusdomain.ProxyCenterStatus, error)

	CreateSubscription(ctx context.Context, subscription *adminplusdomain.ProxySubscription, urlCiphertext string) (*adminplusdomain.ProxySubscription, error)
	UpdateSubscription(ctx context.Context, id int64, input UpdateSubscriptionInput) (*adminplusdomain.ProxySubscription, error)
	DeleteSubscription(ctx context.Context, id int64) error
	GetSubscription(ctx context.Context, id int64) (*adminplusdomain.ProxySubscription, error)
	GetSubscriptionSecret(ctx context.Context, id int64) (*adminplusdomain.ProxySubscription, string, error)
	ListSubscriptions(ctx context.Context, filter SubscriptionFilter) ([]*adminplusdomain.ProxySubscription, error)
	SaveConfigVersion(ctx context.Context, subscriptionID int64, configVersion string, mihomoYAML []byte, generatedAt time.Time) error
	GetConfigVersion(ctx context.Context, subscriptionID int64, configVersion string) ([]byte, error)
	ReplaceNodes(ctx context.Context, subscriptionID int64, configVersion string, nodes []*adminplusdomain.ProxyNode) error
	UpdateSubscriptionRefresh(ctx context.Context, id int64, status adminplusdomain.ProxyRefreshStatus, refreshError string, activeConfigVersion string, nodeCount int, refreshedAt *time.Time) (*adminplusdomain.ProxySubscription, error)

	ListNodes(ctx context.Context, filter NodeFilter) ([]*adminplusdomain.ProxyNode, error)
	GetNode(ctx context.Context, id int64) (*adminplusdomain.ProxyNode, error)
	UpdateNodeHealth(ctx context.Context, id int64, status adminplusdomain.ProxyNodeHealthStatus, latencyMS *int, egressIP string, errorCode string, errorMessage string, checkedAt *time.Time) (*adminplusdomain.ProxyNode, error)
	UpdateNodeDisabled(ctx context.Context, id int64, disabled bool, reason string) (*adminplusdomain.ProxyNode, error)
	RecordHealthCheck(ctx context.Context, check *adminplusdomain.ProxyHealthCheck) (*adminplusdomain.ProxyHealthCheck, error)

	ListPolicies(ctx context.Context, filter PolicyFilter) ([]*adminplusdomain.ProxyPolicy, error)
	GetPolicy(ctx context.Context, id int64) (*adminplusdomain.ProxyPolicy, error)
	CreatePolicy(ctx context.Context, policy *adminplusdomain.ProxyPolicy) (*adminplusdomain.ProxyPolicy, error)
	UpdatePolicy(ctx context.Context, id int64, input UpdatePolicyInput) (*adminplusdomain.ProxyPolicy, error)
	DeletePolicy(ctx context.Context, id int64) error
	ListTargets(ctx context.Context, filter TargetFilter) ([]*adminplusdomain.ProxyTargetPolicy, error)
	CreateTarget(ctx context.Context, target *adminplusdomain.ProxyTargetPolicy) (*adminplusdomain.ProxyTargetPolicy, error)
	UpdateTarget(ctx context.Context, policyID int64, targetID int64, input UpdateTargetInput) (*adminplusdomain.ProxyTargetPolicy, error)
	DeleteTarget(ctx context.Context, policyID int64, targetID int64) error

	ListRuntimeSlots(ctx context.Context, filter RuntimeSlotFilter) ([]*adminplusdomain.ProxyRuntimeSlot, error)
	CreateRuntimeSlot(ctx context.Context, slot *adminplusdomain.ProxyRuntimeSlot, controllerSecretCiphertext string) (*adminplusdomain.ProxyRuntimeSlot, error)
	GetRuntimeSlotSecret(ctx context.Context, id int64) (*adminplusdomain.ProxyRuntimeSlot, string, error)
	UpdateRuntimeSlot(ctx context.Context, id int64, input UpdateRuntimeSlotInput) (*adminplusdomain.ProxyRuntimeSlot, error)

	CreateAssignment(ctx context.Context, assignment *adminplusdomain.ProxyAssignment) (*adminplusdomain.ProxyAssignment, error)
	GetAssignment(ctx context.Context, id int64) (*adminplusdomain.ProxyAssignment, error)
	ListAssignments(ctx context.Context, filter AssignmentFilter) ([]*adminplusdomain.ProxyAssignment, error)
	UpdateAssignment(ctx context.Context, id int64, input UpdateAssignmentInput) (*adminplusdomain.ProxyAssignment, error)

	CreateAuditEvent(ctx context.Context, event *adminplusdomain.ProxyAuditEvent) (*adminplusdomain.ProxyAuditEvent, error)
	ListAuditEvents(ctx context.Context, filter AuditFilter) ([]*adminplusdomain.ProxyAuditEvent, error)

	CountPoliciesUsingSubscription(ctx context.Context, subscriptionID int64) (int, error)
	CountActiveAssignmentsForSubscription(ctx context.Context, subscriptionID int64) (int, error)
	CountActiveAssignmentsForPolicy(ctx context.Context, policyID int64) (int, error)
}

type SubscriptionFilter struct {
	Enabled *bool
	Limit   int
}

type NodeFilter struct {
	SubscriptionID  int64
	SubscriptionIDs []int64
	HealthStatus    adminplusdomain.ProxyNodeHealthStatus
	IncludeDisabled bool
	Query           string
	Limit           int
}

type PolicyFilter struct {
	Enabled *bool
	Limit   int
}

type TargetFilter struct {
	PolicyID int64
	Purpose  adminplusdomain.ProxyTaskPurpose
	Enabled  *bool
	Limit    int
}

type RuntimeSlotFilter struct {
	Status adminplusdomain.ProxyRuntimeSlotStatus
	Limit  int
}

type AssignmentFilter struct {
	TaskType string
	TaskID   string
	Status   adminplusdomain.ProxyAssignmentStatus
	Limit    int
}

type AuditFilter struct {
	EventType  string
	TaskType   string
	TaskID     string
	Level      adminplusdomain.ProxyAuditLevel
	TargetHost string
	Limit      int
}

type UpdateSubscriptionInput struct {
	Name                      *string
	SubscriptionType          *adminplusdomain.ProxySubscriptionType
	SubscriptionURLCiphertext *string
	SubscriptionURLHash       *string
	Enabled                   *bool
	RefreshIntervalSeconds    *int
}

type UpdatePolicyInput struct {
	Name               *string
	Enabled            *bool
	SubscriptionIDs    []int64
	PreferredRegions   []string
	MaxConcurrency     *int
	MaxSwitchesPerTask *int
	ConnectTimeoutMS   *int
	RequestTimeoutMS   *int
	Config             map[string]any
}

type UpdateTargetInput struct {
	TargetHost         *string
	Purpose            *adminplusdomain.ProxyTaskPurpose
	AllowedMethods     []string
	RateLimitPerMinute *int
	Enabled            *bool
	AuthorizationNote  *string
}

type UpdateRuntimeSlotInput struct {
	Status                     *adminplusdomain.ProxyRuntimeSlotStatus
	MixedPort                  *int
	ControllerPort             *int
	ControllerSecretCiphertext *string
	ProcessID                  **int
	ConfigPath                 *string
	AssignedTaskType           *string
	AssignedTaskID             *string
	SelectedNodeID             *int64
	LastStartedAt              **time.Time
	LastHeartbeatAt            **time.Time
}

type UpdateAssignmentInput struct {
	NodeID       *int64
	EgressIP     *string
	Status       *adminplusdomain.ProxyAssignmentStatus
	SwitchCount  *int
	ErrorCode    *string
	ErrorMessage *string
	ReleasedAt   **time.Time
}

type CreateSubscriptionInput struct {
	Name                   string
	SubscriptionType       adminplusdomain.ProxySubscriptionType
	SubscriptionURL        string
	Enabled                bool
	RefreshIntervalSeconds int
	CreatedBy              int64
	RefreshNow             bool
}

type CreatePolicyInput struct {
	Name               string
	Enabled            bool
	SubscriptionIDs    []int64
	PreferredRegions   []string
	MaxConcurrency     int
	MaxSwitchesPerTask int
	ConnectTimeoutMS   int
	RequestTimeoutMS   int
	Config             map[string]any
}

type CreateTargetInput struct {
	PolicyID           int64
	TargetHost         string
	Purpose            adminplusdomain.ProxyTaskPurpose
	AllowedMethods     []string
	RateLimitPerMinute int
	Enabled            bool
	AuthorizationNote  string
}

type RequestAssignmentInput struct {
	TaskType   string
	TaskID     string
	PolicyID   int64
	TargetHost string
	Purpose    adminplusdomain.ProxyTaskPurpose
	Method     string
}

type SwitchAssignmentInput struct {
	NodeID       int64
	ErrorCode    string
	ErrorMessage string
}

type Service struct {
	repo       Repository
	cipher     SecretCipher
	normalizer *SubscriptionNormalizer
	runtime    Runtime
	runtimeCfg RuntimeConfig
	client     *http.Client
	now        func() time.Time
}

func NewService(repo Repository, cipher SecretCipher, normalizer *SubscriptionNormalizer, runtime Runtime, runtimeCfg RuntimeConfig) *Service {
	if normalizer == nil {
		normalizer = NewSubscriptionNormalizer()
	}
	if runtime == nil {
		runtime = NewLocalMihomoRuntime(runtimeCfg)
	}
	return &Service{
		repo:       repo,
		cipher:     cipher,
		normalizer: normalizer,
		runtime:    runtime,
		runtimeCfg: runtimeCfg,
		client:     &http.Client{Timeout: 20 * time.Second},
		now:        time.Now,
	}
}

func (s *Service) CenterStatus(ctx context.Context) (*adminplusdomain.ProxyCenterStatus, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	return s.repo.CenterStatus(ctx)
}

func (s *Service) ListSubscriptions(ctx context.Context, filter SubscriptionFilter) ([]*adminplusdomain.ProxySubscription, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	return s.repo.ListSubscriptions(ctx, filter)
}

func (s *Service) CreateSubscription(ctx context.Context, input CreateSubscriptionInput) (*adminplusdomain.ProxySubscription, error) {
	if s == nil || s.repo == nil || s.cipher == nil {
		return nil, internalError("proxy service is not configured")
	}
	input.Name = strings.TrimSpace(input.Name)
	input.SubscriptionURL = strings.TrimSpace(input.SubscriptionURL)
	if input.Name == "" {
		return nil, invalidInput("PROXY_SUBSCRIPTION_NAME_REQUIRED", "subscription name is required")
	}
	if input.SubscriptionURL == "" {
		return nil, invalidInput("PROXY_SUBSCRIPTION_URL_REQUIRED", "subscription url is required")
	}
	if input.RefreshIntervalSeconds <= 0 {
		input.RefreshIntervalSeconds = 3600
	}
	if input.RefreshIntervalSeconds < 60 {
		return nil, invalidInput("PROXY_SUBSCRIPTION_REFRESH_INTERVAL_INVALID", "refresh interval must be at least 60 seconds")
	}
	ciphertext, err := s.cipher.Encrypt(input.SubscriptionURL)
	if err != nil {
		return nil, err
	}
	subscription, err := s.repo.CreateSubscription(ctx, &adminplusdomain.ProxySubscription{
		Name:                   input.Name,
		SubscriptionType:       input.SubscriptionType,
		URLConfigured:          true,
		URLHash:                publicHash(input.SubscriptionURL),
		Enabled:                input.Enabled,
		RefreshIntervalSeconds: input.RefreshIntervalSeconds,
		LastRefreshStatus:      adminplusdomain.ProxyRefreshNever,
		CreatedBy:              input.CreatedBy,
	}, ciphertext)
	if err != nil {
		return nil, err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:      "subscription_created",
		ActorID:        input.CreatedBy,
		SubscriptionID: subscription.ID,
		Level:          adminplusdomain.ProxyAuditInfo,
		Message:        "代理订阅已创建",
		Payload: map[string]any{
			"subscription_type": input.SubscriptionType,
			"url_hash":          publicHash(input.SubscriptionURL),
		},
	})
	if input.RefreshNow {
		return s.RefreshSubscription(ctx, subscription.ID)
	}
	return subscription, nil
}

func (s *Service) UpdateSubscription(ctx context.Context, id int64, input UpdateSubscriptionInput) (*adminplusdomain.ProxySubscription, error) {
	if s == nil || s.repo == nil || s.cipher == nil {
		return nil, internalError("proxy service is not configured")
	}
	if id <= 0 {
		return nil, invalidInput("PROXY_SUBSCRIPTION_ID_INVALID", "invalid subscription id")
	}
	if input.SubscriptionURLCiphertext != nil {
		urlValue := strings.TrimSpace(*input.SubscriptionURLCiphertext)
		if urlValue == "" {
			return nil, invalidInput("PROXY_SUBSCRIPTION_URL_REQUIRED", "subscription url is required")
		}
		ciphertext, err := s.cipher.Encrypt(urlValue)
		if err != nil {
			return nil, err
		}
		hash := publicHash(urlValue)
		input.SubscriptionURLCiphertext = &ciphertext
		input.SubscriptionURLHash = &hash
	}
	subscription, err := s.repo.UpdateSubscription(ctx, id, input)
	if err != nil {
		return nil, err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:      "subscription_updated",
		SubscriptionID: id,
		Level:          adminplusdomain.ProxyAuditInfo,
		Message:        "代理订阅已更新",
	})
	return subscription, nil
}

func (s *Service) DeleteSubscription(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return internalError("proxy service is not configured")
	}
	if id <= 0 {
		return invalidInput("PROXY_SUBSCRIPTION_ID_INVALID", "invalid subscription id")
	}
	policies, err := s.repo.CountPoliciesUsingSubscription(ctx, id)
	if err != nil {
		return err
	}
	if policies > 0 {
		return conflict("PROXY_SUBSCRIPTION_IN_USE", "proxy subscription is still referenced by policies")
	}
	active, err := s.repo.CountActiveAssignmentsForSubscription(ctx, id)
	if err != nil {
		return err
	}
	if active > 0 {
		return conflict("PROXY_SUBSCRIPTION_ACTIVE_ASSIGNMENT", "proxy subscription has active assignments")
	}
	subscription, err := s.repo.GetSubscription(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteSubscription(ctx, id); err != nil {
		return err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:      "subscription_deleted",
		SubscriptionID: id,
		Level:          adminplusdomain.ProxyAuditWarning,
		Message:        "代理订阅已删除",
		Payload:        map[string]any{"name": subscription.Name},
	})
	return nil
}

func (s *Service) RefreshSubscription(ctx context.Context, id int64) (*adminplusdomain.ProxySubscription, error) {
	if s == nil || s.repo == nil || s.cipher == nil {
		return nil, internalError("proxy service is not configured")
	}
	subscription, ciphertext, err := s.repo.GetSubscriptionSecret(ctx, id)
	if err != nil {
		return nil, err
	}
	rawURL, err := s.cipher.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	content, err := s.fetchSubscription(ctx, rawURL)
	if err != nil {
		updated, updateErr := s.repo.UpdateSubscriptionRefresh(ctx, id, adminplusdomain.ProxyRefreshFailed, err.Error(), subscription.ActiveConfigVersion, subscription.NodeCount, nil)
		if updateErr != nil {
			return nil, updateErr
		}
		_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
			EventType:      "subscription_refresh_failed",
			SubscriptionID: id,
			Level:          adminplusdomain.ProxyAuditError,
			Message:        "代理订阅刷新失败",
			Payload:        map[string]any{"error": err.Error()},
		})
		return updated, err
	}
	normalized, err := s.normalizer.Normalize(subscription.SubscriptionType, subscription.Name, content)
	if err != nil {
		updated, updateErr := s.repo.UpdateSubscriptionRefresh(ctx, id, adminplusdomain.ProxyRefreshInvalid, err.Error(), subscription.ActiveConfigVersion, subscription.NodeCount, nil)
		if updateErr != nil {
			return nil, updateErr
		}
		_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
			EventType:      "subscription_refresh_invalid",
			SubscriptionID: id,
			Level:          adminplusdomain.ProxyAuditError,
			Message:        "代理订阅解析失败",
			Payload:        map[string]any{"error": err.Error()},
		})
		return updated, err
	}
	if err := s.repo.SaveConfigVersion(ctx, id, normalized.ConfigVersion, normalized.MihomoYAML, normalized.GeneratedAt); err != nil {
		return nil, err
	}
	nodes := make([]*adminplusdomain.ProxyNode, 0, len(normalized.Nodes))
	for _, node := range normalized.Nodes {
		nodes = append(nodes, &adminplusdomain.ProxyNode{
			SubscriptionID: id,
			ConfigVersion:  normalized.ConfigVersion,
			NodeKey:        node.RawHash,
			DisplayName:    node.Name,
			Protocol:       node.Protocol,
			Region:         node.Region,
			ServerHash:     serverHash(node.Server),
			HealthStatus:   adminplusdomain.ProxyNodeHealthUnknown,
			RawMetadata:    node.Metadata,
		})
	}
	if err := s.repo.ReplaceNodes(ctx, id, normalized.ConfigVersion, nodes); err != nil {
		return nil, err
	}
	refreshedAt := s.now()
	updated, err := s.repo.UpdateSubscriptionRefresh(ctx, id, adminplusdomain.ProxyRefreshSucceeded, "", normalized.ConfigVersion, len(nodes), &refreshedAt)
	if err != nil {
		return nil, err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:      "subscription_refreshed",
		SubscriptionID: id,
		Level:          adminplusdomain.ProxyAuditInfo,
		Message:        "代理订阅刷新成功",
		Payload: map[string]any{
			"config_version": normalized.ConfigVersion,
			"node_count":     len(nodes),
		},
	})
	return updated, nil
}

func (s *Service) ListNodes(ctx context.Context, filter NodeFilter) ([]*adminplusdomain.ProxyNode, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	return s.repo.ListNodes(ctx, filter)
}

func (s *Service) CheckNode(ctx context.Context, id int64) (*adminplusdomain.ProxyNode, error) {
	if s == nil || s.repo == nil || s.cipher == nil || s.runtime == nil {
		return nil, internalError("proxy service is not configured")
	}
	if strings.TrimSpace(s.runtimeCfg.BinaryPath) == "" {
		return nil, unavailable("PROXY_MIHOMO_BINARY_NOT_CONFIGURED", "mihomo binary path is required for real proxy node checks")
	}
	node, err := s.repo.GetNode(ctx, id)
	if err != nil {
		return nil, err
	}
	now := s.now()
	slot, controllerSecret, err := s.acquireManualCheckSlot(ctx, node)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = s.runtime.RestartSlot(context.Background(), slot)
		slotStatus := adminplusdomain.ProxyRuntimeSlotIdle
		_, _ = s.repo.UpdateRuntimeSlot(context.Background(), slot.ID, UpdateRuntimeSlotInput{
			Status:           &slotStatus,
			AssignedTaskType: strPtr(""),
			AssignedTaskID:   strPtr(""),
			SelectedNodeID:   int64Ptr(0),
			ProcessID:        ptrToIntPtr(nil),
		})
	}()
	configYAML, err := s.repo.GetConfigVersion(ctx, node.SubscriptionID, node.ConfigVersion)
	if err != nil {
		return nil, err
	}
	started := s.now()
	slotResult, err := s.runtime.ConfigureSlot(ctx, slot, node, configYAML, controllerSecret)
	if err != nil {
		checked, updateErr := s.recordNodeCheckFailure(ctx, node, "PROXY_MIHOMO_START_FAILED", err.Error(), &now)
		if updateErr != nil {
			return nil, updateErr
		}
		return checked, err
	}
	statusAssigned := adminplusdomain.ProxyRuntimeSlotAssigned
	slot, err = s.repo.UpdateRuntimeSlot(ctx, slot.ID, UpdateRuntimeSlotInput{
		Status:           &statusAssigned,
		ConfigPath:       &slotResult.ConfigPath,
		ProcessID:        ptrToIntPtr(slotResult.ProcessID),
		LastStartedAt:    ptrToTimePtr(slotResult.StartedAt),
		LastHeartbeatAt:  ptrToTimePtr(&now),
		AssignedTaskType: strPtr("manual_test"),
		AssignedTaskID:   strPtr("node:"+fmt.Sprint(node.ID)),
		SelectedNodeID:   &node.ID,
	})
	if err != nil {
		return nil, err
	}
	egressIP, err := s.runtime.VerifyEgress(ctx, slot.MixedPort)
	latency := int(s.now().Sub(started).Milliseconds())
	if latency < 0 {
		latency = 0
	}
	if err != nil {
		checked, updateErr := s.recordNodeCheckFailure(ctx, node, "PROXY_EGRESS_VERIFY_FAILED", err.Error(), &now)
		if updateErr != nil {
			return nil, updateErr
		}
		return checked, err
	}
	status := adminplusdomain.ProxyNodeHealthHealthy
	checked, err := s.repo.UpdateNodeHealth(ctx, id, status, &latency, egressIP, "", "", &now)
	if err != nil {
		return nil, err
	}
	_, _ = s.repo.RecordHealthCheck(ctx, &adminplusdomain.ProxyHealthCheck{
		NodeID:   node.ID,
		CheckType: "egress_ip",
		Status:   "succeeded",
		LatencyMS: &latency,
		EgressIP:  egressIP,
		CheckedAt: now,
	})
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:      "node_checked",
		NodeID:         node.ID,
		SubscriptionID: node.SubscriptionID,
		Level:          adminplusdomain.ProxyAuditInfo,
		Message:        "代理节点检测成功",
		Payload:        map[string]any{"latency_ms": latency, "egress_ip": egressIP},
	})
	return checked, nil
}

func (s *Service) UpdateNodeDisabled(ctx context.Context, id int64, disabled bool, reason string) (*adminplusdomain.ProxyNode, error) {
	node, err := s.repo.UpdateNodeDisabled(ctx, id, disabled, strings.TrimSpace(reason))
	if err != nil {
		return nil, err
	}
	eventType := "node_enabled"
	if disabled {
		eventType = "node_disabled"
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:      eventType,
		NodeID:         id,
		SubscriptionID: node.SubscriptionID,
		Level:          adminplusdomain.ProxyAuditWarning,
		Message:        "代理节点状态已更新",
		Payload:        map[string]any{"disabled": disabled, "reason": reason},
	})
	return node, nil
}

func (s *Service) ListPolicies(ctx context.Context, filter PolicyFilter) ([]*adminplusdomain.ProxyPolicy, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	return s.repo.ListPolicies(ctx, filter)
}

func (s *Service) CreatePolicy(ctx context.Context, input CreatePolicyInput) (*adminplusdomain.ProxyPolicy, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	policy := normalizePolicyInput(input)
	created, err := s.repo.CreatePolicy(ctx, policy)
	if err != nil {
		return nil, err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType: "policy_created",
		PolicyID:  created.ID,
		Level:     adminplusdomain.ProxyAuditInfo,
		Message:   "代理策略已创建",
	})
	return created, nil
}

func (s *Service) UpdatePolicy(ctx context.Context, id int64, input UpdatePolicyInput) (*adminplusdomain.ProxyPolicy, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	if id <= 0 {
		return nil, invalidInput("PROXY_POLICY_ID_INVALID", "invalid proxy policy id")
	}
	updated, err := s.repo.UpdatePolicy(ctx, id, input)
	if err != nil {
		return nil, err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType: "policy_updated",
		PolicyID:  id,
		Level:     adminplusdomain.ProxyAuditInfo,
		Message:   "代理策略已更新",
	})
	return updated, nil
}

func (s *Service) DeletePolicy(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return internalError("proxy service is not configured")
	}
	if id <= 0 {
		return invalidInput("PROXY_POLICY_ID_INVALID", "invalid proxy policy id")
	}
	active, err := s.repo.CountActiveAssignmentsForPolicy(ctx, id)
	if err != nil {
		return err
	}
	if active > 0 {
		return conflict("PROXY_POLICY_ACTIVE_ASSIGNMENT", "proxy policy has active assignments")
	}
	policy, err := s.repo.GetPolicy(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeletePolicy(ctx, id); err != nil {
		return err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType: "policy_deleted",
		PolicyID:  id,
		Level:     adminplusdomain.ProxyAuditWarning,
		Message:   "代理策略已删除",
		Payload:   map[string]any{"name": policy.Name},
	})
	return nil
}

func (s *Service) ListTargets(ctx context.Context, filter TargetFilter) ([]*adminplusdomain.ProxyTargetPolicy, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	return s.repo.ListTargets(ctx, filter)
}

func (s *Service) CreateTarget(ctx context.Context, input CreateTargetInput) (*adminplusdomain.ProxyTargetPolicy, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	if input.PolicyID <= 0 {
		return nil, invalidInput("PROXY_POLICY_ID_REQUIRED", "proxy policy id is required")
	}
	targetHost := canonicalHost(input.TargetHost)
	if targetHost == "" {
		return nil, invalidInput("PROXY_TARGET_HOST_REQUIRED", "target host is required")
	}
	if input.RateLimitPerMinute <= 0 {
		input.RateLimitPerMinute = 60
	}
	methods := normalizeMethods(input.AllowedMethods)
	target, err := s.repo.CreateTarget(ctx, &adminplusdomain.ProxyTargetPolicy{
		PolicyID:           input.PolicyID,
		TargetHost:         targetHost,
		Purpose:            input.Purpose,
		AllowedMethods:     methods,
		RateLimitPerMinute: input.RateLimitPerMinute,
		Enabled:            input.Enabled,
		AuthorizationNote:  strings.TrimSpace(input.AuthorizationNote),
	})
	if err != nil {
		return nil, err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:  "target_policy_created",
		PolicyID:   input.PolicyID,
		TargetHost: targetHost,
		Level:      adminplusdomain.ProxyAuditInfo,
		Message:    "代理目标白名单已创建",
	})
	return target, nil
}

func (s *Service) UpdateTarget(ctx context.Context, policyID int64, targetID int64, input UpdateTargetInput) (*adminplusdomain.ProxyTargetPolicy, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	if policyID <= 0 || targetID <= 0 {
		return nil, invalidInput("PROXY_TARGET_POLICY_ID_INVALID", "invalid proxy target policy id")
	}
	if input.TargetHost != nil {
		value := canonicalHost(*input.TargetHost)
		if value == "" {
			return nil, invalidInput("PROXY_TARGET_HOST_REQUIRED", "target host is required")
		}
		input.TargetHost = &value
	}
	if input.AllowedMethods != nil {
		input.AllowedMethods = normalizeMethods(input.AllowedMethods)
	}
	if input.AuthorizationNote != nil {
		value := strings.TrimSpace(*input.AuthorizationNote)
		input.AuthorizationNote = &value
	}
	target, err := s.repo.UpdateTarget(ctx, policyID, targetID, input)
	if err != nil {
		return nil, err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:  "target_policy_updated",
		PolicyID:   policyID,
		TargetHost: target.TargetHost,
		Level:      adminplusdomain.ProxyAuditInfo,
		Message:    "代理目标白名单已更新",
	})
	return target, nil
}

func (s *Service) DeleteTarget(ctx context.Context, policyID int64, targetID int64) error {
	if s == nil || s.repo == nil {
		return internalError("proxy service is not configured")
	}
	if policyID <= 0 || targetID <= 0 {
		return invalidInput("PROXY_TARGET_POLICY_ID_INVALID", "invalid proxy target policy id")
	}
	if err := s.repo.DeleteTarget(ctx, policyID, targetID); err != nil {
		return err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType: "target_policy_deleted",
		PolicyID:  policyID,
		Level:     adminplusdomain.ProxyAuditWarning,
		Message:   "代理目标白名单已删除",
		Payload:   map[string]any{"target_id": targetID},
	})
	return nil
}

func (s *Service) ListRuntimeSlots(ctx context.Context, filter RuntimeSlotFilter) ([]*adminplusdomain.ProxyRuntimeSlot, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("proxy service is not configured")
	}
	return s.repo.ListRuntimeSlots(ctx, filter)
}

func (s *Service) RestartSlot(ctx context.Context, id int64) (*adminplusdomain.ProxyRuntimeSlot, error) {
	if s == nil || s.repo == nil || s.runtime == nil {
		return nil, internalError("proxy service is not configured")
	}
	slot, _, err := s.repo.GetRuntimeSlotSecret(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.runtime.RestartSlot(ctx, slot); err != nil {
		return nil, err
	}
	status := adminplusdomain.ProxyRuntimeSlotStopped
	updated, err := s.repo.UpdateRuntimeSlot(ctx, id, UpdateRuntimeSlotInput{
		Status:           &status,
		AssignedTaskType: strPtr(""),
		AssignedTaskID:   strPtr(""),
		SelectedNodeID:   int64Ptr(0),
		ProcessID:        ptrToIntPtr(nil),
	})
	if err != nil {
		return nil, err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType: "slot_restarted",
		SlotID:    id,
		Level:     adminplusdomain.ProxyAuditWarning,
		Message:   "代理运行槽位已重启",
	})
	return updated, nil
}

func (s *Service) RequestAssignment(ctx context.Context, input RequestAssignmentInput) (*adminplusdomain.ProxyAssignment, error) {
	if s == nil || s.repo == nil || s.cipher == nil || s.runtime == nil {
		return nil, internalError("proxy service is not configured")
	}
	input.TaskType = strings.TrimSpace(input.TaskType)
	input.TaskID = strings.TrimSpace(input.TaskID)
	input.TargetHost = canonicalHost(input.TargetHost)
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	if input.Method == "" {
		input.Method = http.MethodGet
	}
	if input.TaskType == "" || input.TaskID == "" {
		return nil, invalidInput("PROXY_TASK_REQUIRED", "task type and task id are required")
	}
	policy, err := s.repo.GetPolicy(ctx, input.PolicyID)
	if err != nil {
		return nil, err
	}
	if !policy.Enabled {
		return nil, forbidden("PROXY_POLICY_DISABLED", "proxy policy is disabled")
	}
	target, err := s.findAllowedTarget(ctx, policy.ID, input.TargetHost, input.Purpose, input.Method)
	if err != nil {
		_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
			EventType:  "policy_denied",
			TaskType:   input.TaskType,
			TaskID:     input.TaskID,
			PolicyID:   policy.ID,
			TargetHost: input.TargetHost,
			Level:      adminplusdomain.ProxyAuditWarning,
			Message:    "代理策略拒绝目标访问",
			Payload:    map[string]any{"reason": infraerrors.Reason(err)},
		})
		return nil, err
	}
	node, err := s.selectNode(ctx, policy)
	if err != nil {
		return nil, err
	}
	slot, controllerSecret, err := s.acquireSlot(ctx, policy, input.TaskType, input.TaskID, node.ID)
	if err != nil {
		return nil, err
	}
	configYAML, err := s.repo.GetConfigVersion(ctx, node.SubscriptionID, node.ConfigVersion)
	if err != nil {
		return nil, err
	}
	slotResult, err := s.runtime.ConfigureSlot(ctx, slot, node, configYAML, controllerSecret)
	if err != nil {
		return nil, err
	}
	status := adminplusdomain.ProxyRuntimeSlotAssigned
	now := s.now()
	slot, err = s.repo.UpdateRuntimeSlot(ctx, slot.ID, UpdateRuntimeSlotInput{
		Status:           &status,
		ConfigPath:       &slotResult.ConfigPath,
		ProcessID:        ptrToIntPtr(slotResult.ProcessID),
		LastStartedAt:    ptrToTimePtr(slotResult.StartedAt),
		LastHeartbeatAt:  ptrToTimePtr(&now),
		AssignedTaskType: &input.TaskType,
		AssignedTaskID:   &input.TaskID,
		SelectedNodeID:   &node.ID,
	})
	if err != nil {
		return nil, err
	}
	egressIP := ""
	if s.runtimeCfg.BinaryPath != "" {
		if ip, verifyErr := s.runtime.VerifyEgress(ctx, slot.MixedPort); verifyErr == nil {
			egressIP = ip
		}
	}
	assignment, err := s.repo.CreateAssignment(ctx, &adminplusdomain.ProxyAssignment{
		TaskType:   input.TaskType,
		TaskID:     input.TaskID,
		PolicyID:   policy.ID,
		SlotID:     slot.ID,
		NodeID:     node.ID,
		TargetHost: target.TargetHost,
		EgressIP:   egressIP,
		Status:     adminplusdomain.ProxyAssignmentActive,
		StartedAt:  now,
	})
	if err != nil {
		return nil, err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:  "assignment_created",
		TaskType:   input.TaskType,
		TaskID:     input.TaskID,
		PolicyID:   policy.ID,
		SlotID:     slot.ID,
		NodeID:     node.ID,
		TargetHost: input.TargetHost,
		Level:      adminplusdomain.ProxyAuditInfo,
		Message:    "代理任务绑定已创建",
		Payload: map[string]any{
			"mixed_port": slot.MixedPort,
			"node_name":  node.DisplayName,
			"egress_ip":  egressIP,
		},
	})
	return assignment, nil
}

func (s *Service) ReleaseAssignment(ctx context.Context, id int64, failed bool, errorCode string, errorMessage string) (*adminplusdomain.ProxyAssignment, error) {
	assignment, err := s.repo.GetAssignment(ctx, id)
	if err != nil {
		return nil, err
	}
	status := adminplusdomain.ProxyAssignmentReleased
	if failed {
		status = adminplusdomain.ProxyAssignmentFailed
	}
	releasedAt := s.now()
	updated, err := s.repo.UpdateAssignment(ctx, id, UpdateAssignmentInput{
		Status:       &status,
		ErrorCode:    &errorCode,
		ErrorMessage: &errorMessage,
		ReleasedAt:   ptrToTimePtr(&releasedAt),
	})
	if err != nil {
		return nil, err
	}
	slotStatus := adminplusdomain.ProxyRuntimeSlotIdle
	if slot, _, slotErr := s.repo.GetRuntimeSlotSecret(ctx, assignment.SlotID); slotErr == nil {
		_ = s.runtime.RestartSlot(context.Background(), slot)
	}
	_, _ = s.repo.UpdateRuntimeSlot(ctx, assignment.SlotID, UpdateRuntimeSlotInput{
		Status:           &slotStatus,
		AssignedTaskType: strPtr(""),
		AssignedTaskID:   strPtr(""),
		SelectedNodeID:   int64Ptr(0),
		ProcessID:        ptrToIntPtr(nil),
	})
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:  "assignment_released",
		TaskType:   assignment.TaskType,
		TaskID:     assignment.TaskID,
		PolicyID:   assignment.PolicyID,
		SlotID:     assignment.SlotID,
		NodeID:     assignment.NodeID,
		TargetHost: assignment.TargetHost,
		Level:      adminplusdomain.ProxyAuditInfo,
		Message:    "代理任务绑定已释放",
		Payload:    map[string]any{"failed": failed, "error_code": errorCode},
	})
	return updated, nil
}

func (s *Service) SwitchAssignment(ctx context.Context, id int64, input SwitchAssignmentInput) (*adminplusdomain.ProxyAssignment, error) {
	assignment, err := s.repo.GetAssignment(ctx, id)
	if err != nil {
		return nil, err
	}
	if assignment.Status != adminplusdomain.ProxyAssignmentActive {
		return nil, conflict("PROXY_ASSIGNMENT_NOT_ACTIVE", "proxy assignment is not active")
	}
	node, err := s.repo.GetNode(ctx, input.NodeID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureNodeAllowedByAssignment(ctx, assignment, node); err != nil {
		return nil, err
	}
	slot, secretCiphertext, err := s.repo.GetRuntimeSlotSecret(ctx, assignment.SlotID)
	if err != nil {
		return nil, err
	}
	controllerSecret := ""
	if secretCiphertext != "" && s.cipher != nil {
		controllerSecret, _ = s.cipher.Decrypt(secretCiphertext)
	}
	if s.runtimeCfg.BinaryPath != "" {
		if err := s.runtime.SwitchNode(ctx, slot, node.DisplayName, controllerSecret); err != nil {
			return nil, err
		}
	}
	egressIP := assignment.EgressIP
	if s.runtimeCfg.BinaryPath != "" {
		if ip, verifyErr := s.runtime.VerifyEgress(ctx, slot.MixedPort); verifyErr == nil {
			egressIP = ip
		}
	}
	switchCount := assignment.SwitchCount + 1
	updated, err := s.repo.UpdateAssignment(ctx, id, UpdateAssignmentInput{
		NodeID:       &node.ID,
		EgressIP:     &egressIP,
		SwitchCount:  &switchCount,
		ErrorCode:    &input.ErrorCode,
		ErrorMessage: &input.ErrorMessage,
	})
	if err != nil {
		return nil, err
	}
	_, _ = s.repo.UpdateRuntimeSlot(ctx, slot.ID, UpdateRuntimeSlotInput{SelectedNodeID: &node.ID})
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:  "node_switched",
		TaskType:   assignment.TaskType,
		TaskID:     assignment.TaskID,
		PolicyID:   assignment.PolicyID,
		SlotID:     slot.ID,
		NodeID:     node.ID,
		TargetHost: assignment.TargetHost,
		Level:      adminplusdomain.ProxyAuditWarning,
		Message:    "代理节点已切换",
		Payload: map[string]any{
			"switch_count":  switchCount,
			"previous_node": assignment.NodeID,
			"error_code":    input.ErrorCode,
		},
	})
	return updated, nil
}

func (s *Service) ensureNodeAllowedByAssignment(ctx context.Context, assignment *adminplusdomain.ProxyAssignment, node *adminplusdomain.ProxyNode) error {
	if assignment == nil || node == nil {
		return invalidInput("PROXY_ASSIGNMENT_NODE_REQUIRED", "proxy assignment and node are required")
	}
	if node.HealthStatus == adminplusdomain.ProxyNodeHealthDisabled ||
		node.HealthStatus == adminplusdomain.ProxyNodeHealthUnhealthy ||
		node.HealthStatus == adminplusdomain.ProxyNodeHealthSuspect {
		return unavailable("PROXY_NODE_UNAVAILABLE", "proxy node is not available")
	}
	policy, err := s.repo.GetPolicy(ctx, assignment.PolicyID)
	if err != nil {
		return err
	}
	if !containsInt64(policy.SubscriptionIDs, node.SubscriptionID) {
		return forbidden("PROXY_NODE_POLICY_MISMATCH", "proxy node does not belong to assignment policy subscriptions")
	}
	return nil
}

func (s *Service) ListAssignments(ctx context.Context, filter AssignmentFilter) ([]*adminplusdomain.ProxyAssignment, error) {
	return s.repo.ListAssignments(ctx, filter)
}

func (s *Service) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]*adminplusdomain.ProxyAuditEvent, error) {
	return s.repo.ListAuditEvents(ctx, filter)
}

func (s *Service) fetchSubscription(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, invalidInput("PROXY_SUBSCRIPTION_URL_INVALID", "invalid subscription url").WithCause(err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, unavailable("PROXY_SUBSCRIPTION_FETCH_FAILED", "failed to fetch proxy subscription").WithCause(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, unavailable("PROXY_SUBSCRIPTION_FETCH_FAILED", fmt.Sprintf("subscription endpoint returned HTTP %d", resp.StatusCode))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func (s *Service) findAllowedTarget(ctx context.Context, policyID int64, targetHost string, purpose adminplusdomain.ProxyTaskPurpose, method string) (*adminplusdomain.ProxyTargetPolicy, error) {
	if targetHost == "" {
		return nil, invalidInput("PROXY_TARGET_HOST_REQUIRED", "target host is required")
	}
	enabled := true
	targets, err := s.repo.ListTargets(ctx, TargetFilter{PolicyID: policyID, Purpose: purpose, Enabled: &enabled, Limit: 1000})
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		if !hostMatchesPolicy(targetHost, target.TargetHost) {
			continue
		}
		if methodAllowed(method, target.AllowedMethods) {
			return target, nil
		}
		return nil, forbidden("PROXY_POLICY_METHOD_DENIED", "request method is not allowed by proxy target policy")
	}
	return nil, forbidden("PROXY_POLICY_TARGET_DENIED", "target host is not allowed by proxy policy")
}

func (s *Service) selectNode(ctx context.Context, policy *adminplusdomain.ProxyPolicy) (*adminplusdomain.ProxyNode, error) {
	nodes, err := s.repo.ListNodes(ctx, NodeFilter{
		SubscriptionIDs: policy.SubscriptionIDs,
		IncludeDisabled: false,
		Limit:           1000,
	})
	if err != nil {
		return nil, err
	}
	candidates := make([]*adminplusdomain.ProxyNode, 0, len(nodes))
	for _, node := range nodes {
		if node.HealthStatus == adminplusdomain.ProxyNodeHealthDisabled ||
			node.HealthStatus == adminplusdomain.ProxyNodeHealthUnhealthy ||
			node.HealthStatus == adminplusdomain.ProxyNodeHealthSuspect {
			continue
		}
		candidates = append(candidates, node)
	}
	if len(candidates) == 0 {
		return nil, unavailable("PROXY_NODE_NO_HEALTHY_NODE", "no usable proxy node is available")
	}
	if fixedNodeID := fixedNodeIDFromPolicy(policy); fixedNodeID > 0 {
		for _, node := range candidates {
			if node.ID == fixedNodeID {
				return node, nil
			}
		}
		return nil, unavailable("PROXY_FIXED_NODE_UNAVAILABLE", "configured fixed proxy node is not available")
	}
	regionRank := make(map[string]int, len(policy.PreferredRegions))
	for i, region := range policy.PreferredRegions {
		regionRank[strings.ToUpper(strings.TrimSpace(region))] = i
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		leftHealthy := left.HealthStatus == adminplusdomain.ProxyNodeHealthHealthy
		rightHealthy := right.HealthStatus == adminplusdomain.ProxyNodeHealthHealthy
		if leftHealthy != rightHealthy {
			return leftHealthy
		}
		leftRank, leftOK := regionRank[strings.ToUpper(left.Region)]
		rightRank, rightOK := regionRank[strings.ToUpper(right.Region)]
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK && rightOK && leftRank != rightRank {
			return leftRank < rightRank
		}
		if left.LastLatencyMS != nil && right.LastLatencyMS != nil && *left.LastLatencyMS != *right.LastLatencyMS {
			return *left.LastLatencyMS < *right.LastLatencyMS
		}
		return left.ID < right.ID
	})
	return candidates[0], nil
}

func fixedNodeIDFromPolicy(policy *adminplusdomain.ProxyPolicy) int64 {
	if policy == nil || strings.TrimSpace(fmt.Sprint(policy.Config["selection_mode"])) != "fixed" {
		return 0
	}
	switch value := policy.Config["fixed_node_id"].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case json.Number:
		id, _ := value.Int64()
		return id
	case string:
		id, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return id
	default:
		return 0
	}
}

func (s *Service) acquireSlot(ctx context.Context, policy *adminplusdomain.ProxyPolicy, taskType string, taskID string, nodeID int64) (*adminplusdomain.ProxyRuntimeSlot, string, error) {
	slots, err := s.repo.ListRuntimeSlots(ctx, RuntimeSlotFilter{Limit: s.runtimeCfg.MaxSlots + 10})
	if err != nil {
		return nil, "", err
	}
	for _, slot := range slots {
		if slot.Status == adminplusdomain.ProxyRuntimeSlotIdle || slot.Status == adminplusdomain.ProxyRuntimeSlotStopped {
			return s.assignSlotSecret(ctx, slot, taskType, taskID, nodeID)
		}
	}
	if len(slots) >= policy.MaxConcurrency || len(slots) >= s.runtimeCfg.MaxSlots {
		return nil, "", unavailable("PROXY_SLOT_EXHAUSTED", "no proxy runtime slot is available")
	}
	index := len(slots) + 1
	secret := randomSecret()
	ciphertext, err := s.cipher.Encrypt(secret)
	if err != nil {
		return nil, "", err
	}
	slot, err := s.repo.CreateRuntimeSlot(ctx, &adminplusdomain.ProxyRuntimeSlot{
		SlotKey:        fmt.Sprintf("proxy-slot-%03d", index),
		Status:         adminplusdomain.ProxyRuntimeSlotIdle,
		MixedPort:      s.runtimeCfg.BaseMixedPort + index - 1,
		ControllerPort: s.runtimeCfg.BaseControllerPort + index - 1,
	}, ciphertext)
	if err != nil {
		return nil, "", err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType: "slot_created",
		SlotID:    slot.ID,
		Level:     adminplusdomain.ProxyAuditInfo,
		Message:   "代理运行槽位已创建",
	})
	return s.assignSlotSecret(ctx, slot, taskType, taskID, nodeID)
}

func (s *Service) acquireManualCheckSlot(ctx context.Context, node *adminplusdomain.ProxyNode) (*adminplusdomain.ProxyRuntimeSlot, string, error) {
	if node == nil {
		return nil, "", invalidInput("PROXY_NODE_REQUIRED", "proxy node is required")
	}
	slots, err := s.repo.ListRuntimeSlots(ctx, RuntimeSlotFilter{Limit: s.runtimeCfg.MaxSlots + 10})
	if err != nil {
		return nil, "", err
	}
	for _, slot := range slots {
		if slot.Status == adminplusdomain.ProxyRuntimeSlotIdle || slot.Status == adminplusdomain.ProxyRuntimeSlotStopped {
			return s.assignSlotSecret(ctx, slot, "manual_test", "node:"+fmt.Sprint(node.ID), node.ID)
		}
	}
	if len(slots) >= s.runtimeCfg.MaxSlots {
		return nil, "", unavailable("PROXY_SLOT_EXHAUSTED", "no proxy runtime slot is available")
	}
	index := len(slots) + 1
	secret := randomSecret()
	ciphertext, err := s.cipher.Encrypt(secret)
	if err != nil {
		return nil, "", err
	}
	slot, err := s.repo.CreateRuntimeSlot(ctx, &adminplusdomain.ProxyRuntimeSlot{
		SlotKey:        fmt.Sprintf("proxy-slot-%03d", index),
		Status:         adminplusdomain.ProxyRuntimeSlotIdle,
		MixedPort:      s.runtimeCfg.BaseMixedPort + index - 1,
		ControllerPort: s.runtimeCfg.BaseControllerPort + index - 1,
	}, ciphertext)
	if err != nil {
		return nil, "", err
	}
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType: "slot_created",
		SlotID:    slot.ID,
		Level:     adminplusdomain.ProxyAuditInfo,
		Message:   "代理运行槽位已创建",
	})
	return s.assignSlotSecret(ctx, slot, "manual_test", "node:"+fmt.Sprint(node.ID), node.ID)
}

func (s *Service) recordNodeCheckFailure(ctx context.Context, node *adminplusdomain.ProxyNode, code string, message string, checkedAt *time.Time) (*adminplusdomain.ProxyNode, error) {
	if node == nil {
		return nil, invalidInput("PROXY_NODE_REQUIRED", "proxy node is required")
	}
	status := adminplusdomain.ProxyNodeHealthUnhealthy
	checked, err := s.repo.UpdateNodeHealth(ctx, node.ID, status, nil, "", code, trimLimit(message, 500), checkedAt)
	if err != nil {
		return nil, err
	}
	when := s.now()
	if checkedAt != nil {
		when = *checkedAt
	}
	_, _ = s.repo.RecordHealthCheck(ctx, &adminplusdomain.ProxyHealthCheck{
		NodeID:       node.ID,
		CheckType:    "egress_ip",
		Status:       "failed",
		ErrorCode:    code,
		ErrorMessage: trimLimit(message, 500),
		CheckedAt:    when,
	})
	_, _ = s.audit(ctx, &adminplusdomain.ProxyAuditEvent{
		EventType:      "node_check_failed",
		NodeID:         node.ID,
		SubscriptionID: node.SubscriptionID,
		Level:          adminplusdomain.ProxyAuditError,
		Message:        "代理节点检测失败",
		Payload:        map[string]any{"error_code": code, "error_message": trimLimit(message, 500)},
	})
	return checked, nil
}

func (s *Service) assignSlotSecret(ctx context.Context, slot *adminplusdomain.ProxyRuntimeSlot, taskType string, taskID string, nodeID int64) (*adminplusdomain.ProxyRuntimeSlot, string, error) {
	_, ciphertext, err := s.repo.GetRuntimeSlotSecret(ctx, slot.ID)
	if err != nil {
		return nil, "", err
	}
	secret := ""
	if ciphertext != "" {
		secret, _ = s.cipher.Decrypt(ciphertext)
	}
	if secret == "" {
		secret = randomSecret()
		newCiphertext, err := s.cipher.Encrypt(secret)
		if err != nil {
			return nil, "", err
		}
		_, err = s.repo.UpdateRuntimeSlot(ctx, slot.ID, UpdateRuntimeSlotInput{ControllerSecretCiphertext: &newCiphertext})
		if err != nil {
			return nil, "", err
		}
	}
	status := adminplusdomain.ProxyRuntimeSlotAssigned
	updated, err := s.repo.UpdateRuntimeSlot(ctx, slot.ID, UpdateRuntimeSlotInput{
		Status:           &status,
		AssignedTaskType: &taskType,
		AssignedTaskID:   &taskID,
		SelectedNodeID:   &nodeID,
	})
	return updated, secret, err
}

func normalizePolicyInput(input CreatePolicyInput) *adminplusdomain.ProxyPolicy {
	if input.MaxConcurrency <= 0 {
		input.MaxConcurrency = 1
	}
	if input.MaxSwitchesPerTask < 0 {
		input.MaxSwitchesPerTask = 0
	}
	if input.ConnectTimeoutMS <= 0 {
		input.ConnectTimeoutMS = 10000
	}
	if input.RequestTimeoutMS <= 0 {
		input.RequestTimeoutMS = 30000
	}
	return &adminplusdomain.ProxyPolicy{
		Name:               strings.TrimSpace(input.Name),
		Enabled:            input.Enabled,
		SubscriptionIDs:    dedupeInt64(input.SubscriptionIDs),
		PreferredRegions:   dedupeStrings(input.PreferredRegions),
		MaxConcurrency:     input.MaxConcurrency,
		MaxSwitchesPerTask: input.MaxSwitchesPerTask,
		ConnectTimeoutMS:   input.ConnectTimeoutMS,
		RequestTimeoutMS:   input.RequestTimeoutMS,
		Config:             input.Config,
	}
}

func normalizeMethods(methods []string) []string {
	out := make([]string, 0, len(methods))
	for _, method := range methods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method != "" {
			out = append(out, method)
		}
	}
	if len(out) == 0 {
		return []string{http.MethodGet, http.MethodPost}
	}
	return dedupeStrings(out)
}

func methodAllowed(method string, allowed []string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	for _, item := range allowed {
		if strings.ToUpper(strings.TrimSpace(item)) == method {
			return true
		}
	}
	return false
}

func (s *Service) audit(ctx context.Context, event *adminplusdomain.ProxyAuditEvent) (*adminplusdomain.ProxyAuditEvent, error) {
	if s == nil || s.repo == nil || event == nil {
		return nil, nil
	}
	if event.Level == "" {
		event.Level = adminplusdomain.ProxyAuditInfo
	}
	return s.repo.CreateAuditEvent(ctx, event)
}

func dedupeInt64(values []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToUpper(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func randomSecret() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}

func trimLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func strPtr(value string) *string { return &value }
func int64Ptr(value int64) *int64 { return &value }

func ptrToIntPtr(value *int) **int {
	return &value
}

func ptrToTimePtr(value *time.Time) **time.Time {
	return &value
}

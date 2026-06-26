package domain

import "time"

type ProxySubscriptionType string

const (
	ProxySubscriptionClash        ProxySubscriptionType = "clash"
	ProxySubscriptionShadowrocket ProxySubscriptionType = "shadowrocket"
	ProxySubscriptionV2RaySS      ProxySubscriptionType = "v2ray_ss"
)

type ProxyRefreshStatus string

const (
	ProxyRefreshNever     ProxyRefreshStatus = "never"
	ProxyRefreshSucceeded ProxyRefreshStatus = "succeeded"
	ProxyRefreshFailed    ProxyRefreshStatus = "failed"
	ProxyRefreshInvalid   ProxyRefreshStatus = "invalid"
)

type ProxyNodeHealthStatus string

const (
	ProxyNodeHealthUnknown   ProxyNodeHealthStatus = "unknown"
	ProxyNodeHealthHealthy   ProxyNodeHealthStatus = "healthy"
	ProxyNodeHealthDegraded  ProxyNodeHealthStatus = "degraded"
	ProxyNodeHealthSuspect   ProxyNodeHealthStatus = "suspect"
	ProxyNodeHealthUnhealthy ProxyNodeHealthStatus = "unhealthy"
	ProxyNodeHealthDisabled  ProxyNodeHealthStatus = "disabled"
)

type ProxyRuntimeSlotStatus string

const (
	ProxyRuntimeSlotIdle      ProxyRuntimeSlotStatus = "idle"
	ProxyRuntimeSlotAssigned  ProxyRuntimeSlotStatus = "assigned"
	ProxyRuntimeSlotDraining  ProxyRuntimeSlotStatus = "draining"
	ProxyRuntimeSlotUnhealthy ProxyRuntimeSlotStatus = "unhealthy"
	ProxyRuntimeSlotStopped   ProxyRuntimeSlotStatus = "stopped"
)

type ProxyAssignmentStatus string

const (
	ProxyAssignmentActive   ProxyAssignmentStatus = "active"
	ProxyAssignmentReleased ProxyAssignmentStatus = "released"
	ProxyAssignmentFailed   ProxyAssignmentStatus = "failed"
)

type ProxyTaskPurpose string

const (
	ProxyPurposeSiteDiscovery ProxyTaskPurpose = "site_discovery"
	ProxyPurposeRegistration  ProxyTaskPurpose = "registration"
	ProxyPurposeSupplierProbe ProxyTaskPurpose = "supplier_probe"
	ProxyPurposeManualTest    ProxyTaskPurpose = "manual_test"
)

type ProxyAuditLevel string

const (
	ProxyAuditInfo    ProxyAuditLevel = "info"
	ProxyAuditWarning ProxyAuditLevel = "warning"
	ProxyAuditError   ProxyAuditLevel = "error"
)

type ProxySubscription struct {
	ID                     int64                 `json:"id"`
	Name                   string                `json:"name"`
	SubscriptionType       ProxySubscriptionType `json:"subscription_type"`
	URLConfigured          bool                  `json:"url_configured"`
	URLHash                string                `json:"url_hash,omitempty"`
	Enabled                bool                  `json:"enabled"`
	RefreshIntervalSeconds int                   `json:"refresh_interval_seconds"`
	LastRefreshStatus      ProxyRefreshStatus    `json:"last_refresh_status"`
	LastRefreshError       string                `json:"last_refresh_error,omitempty"`
	ActiveConfigVersion    string                `json:"active_config_version,omitempty"`
	NodeCount              int                   `json:"node_count"`
	CreatedBy              int64                 `json:"created_by,omitempty"`
	CreatedAt              time.Time             `json:"created_at"`
	UpdatedAt              time.Time             `json:"updated_at"`
	LastRefreshedAt        *time.Time            `json:"last_refreshed_at,omitempty"`
}

type ProxyNode struct {
	ID               int64                 `json:"id"`
	SubscriptionID   int64                 `json:"subscription_id"`
	ConfigVersion    string                `json:"config_version"`
	NodeKey          string                `json:"node_key"`
	DisplayName      string                `json:"display_name"`
	Protocol         string                `json:"protocol"`
	Region           string                `json:"region,omitempty"`
	ServerHash       string                `json:"server_hash,omitempty"`
	HealthStatus     ProxyNodeHealthStatus `json:"health_status"`
	LastLatencyMS    *int                  `json:"last_latency_ms,omitempty"`
	LastEgressIP     string                `json:"last_egress_ip,omitempty"`
	LastErrorCode    string                `json:"last_error_code,omitempty"`
	LastErrorMessage string                `json:"last_error_message,omitempty"`
	LastCheckedAt    *time.Time            `json:"last_checked_at,omitempty"`
	DisabledReason   string                `json:"disabled_reason,omitempty"`
	RawMetadata      map[string]any        `json:"raw_metadata,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

type ProxyPolicy struct {
	ID                    int64          `json:"id"`
	Name                  string         `json:"name"`
	Enabled               bool           `json:"enabled"`
	SubscriptionIDs       []int64        `json:"subscription_ids"`
	PreferredRegions      []string       `json:"preferred_regions"`
	MaxConcurrency        int            `json:"max_concurrency"`
	MaxSwitchesPerTask    int            `json:"max_switches_per_task"`
	ConnectTimeoutMS      int            `json:"connect_timeout_ms"`
	RequestTimeoutMS      int            `json:"request_timeout_ms"`
	Config                map[string]any `json:"config,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	EnabledTargets        int            `json:"enabled_targets,omitempty"`
	HealthyNodesAvailable int            `json:"healthy_nodes_available,omitempty"`
}

type ProxyTargetPolicy struct {
	ID                 int64            `json:"id"`
	PolicyID           int64            `json:"policy_id"`
	TargetHost         string           `json:"target_host"`
	Purpose            ProxyTaskPurpose `json:"purpose"`
	AllowedMethods     []string         `json:"allowed_methods"`
	RateLimitPerMinute int              `json:"rate_limit_per_minute"`
	Enabled            bool             `json:"enabled"`
	AuthorizationNote  string           `json:"authorization_note,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

type ProxyRuntimeSlot struct {
	ID                         int64                  `json:"id"`
	SlotKey                    string                 `json:"slot_key"`
	Status                     ProxyRuntimeSlotStatus `json:"status"`
	MixedPort                  int                    `json:"mixed_port"`
	ControllerPort             int                    `json:"controller_port"`
	ControllerSecretConfigured bool                   `json:"controller_secret_configured"`
	ProcessID                  *int                   `json:"process_id,omitempty"`
	ConfigPath                 string                 `json:"config_path,omitempty"`
	AssignedTaskType           string                 `json:"assigned_task_type,omitempty"`
	AssignedTaskID             string                 `json:"assigned_task_id,omitempty"`
	SelectedNodeID             int64                  `json:"selected_node_id,omitempty"`
	LastStartedAt              *time.Time             `json:"last_started_at,omitempty"`
	LastHeartbeatAt            *time.Time             `json:"last_heartbeat_at,omitempty"`
	CreatedAt                  time.Time              `json:"created_at"`
	UpdatedAt                  time.Time              `json:"updated_at"`
}

type ProxyAssignment struct {
	ID           int64                 `json:"id"`
	TaskType     string                `json:"task_type"`
	TaskID       string                `json:"task_id"`
	PolicyID     int64                 `json:"policy_id"`
	SlotID       int64                 `json:"slot_id"`
	NodeID       int64                 `json:"node_id,omitempty"`
	MixedPort    int                   `json:"mixed_port,omitempty"`
	TargetHost   string                `json:"target_host"`
	EgressIP     string                `json:"egress_ip,omitempty"`
	Status       ProxyAssignmentStatus `json:"status"`
	SwitchCount  int                   `json:"switch_count"`
	ErrorCode    string                `json:"error_code,omitempty"`
	ErrorMessage string                `json:"error_message,omitempty"`
	StartedAt    time.Time             `json:"started_at"`
	ReleasedAt   *time.Time            `json:"released_at,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
}

type ProxyHealthCheck struct {
	ID           int64     `json:"id"`
	NodeID       int64     `json:"node_id"`
	CheckType    string    `json:"check_type"`
	Status       string    `json:"status"`
	LatencyMS    *int      `json:"latency_ms,omitempty"`
	EgressIP     string    `json:"egress_ip,omitempty"`
	TargetHost   string    `json:"target_host,omitempty"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CheckedAt    time.Time `json:"checked_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type ProxyAuditEvent struct {
	ID             int64           `json:"id"`
	EventType      string          `json:"event_type"`
	ActorID        int64           `json:"actor_id,omitempty"`
	TaskType       string          `json:"task_type,omitempty"`
	TaskID         string          `json:"task_id,omitempty"`
	PolicyID       int64           `json:"policy_id,omitempty"`
	SlotID         int64           `json:"slot_id,omitempty"`
	NodeID         int64           `json:"node_id,omitempty"`
	SubscriptionID int64           `json:"subscription_id,omitempty"`
	TargetHost     string          `json:"target_host,omitempty"`
	Level          ProxyAuditLevel `json:"level"`
	Message        string          `json:"message"`
	Payload        map[string]any  `json:"payload,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type ProxyCenterStatus struct {
	SubscriptionsTotal  int `json:"subscriptions_total"`
	SubscriptionsActive int `json:"subscriptions_active"`
	NodesTotal          int `json:"nodes_total"`
	HealthyNodes        int `json:"healthy_nodes"`
	PoliciesTotal       int `json:"policies_total"`
	TargetsTotal        int `json:"targets_total"`
	SlotsTotal          int `json:"slots_total"`
	SlotsAssigned       int `json:"slots_assigned"`
	AssignmentsActive   int `json:"assignments_active"`
	RecentErrors        int `json:"recent_errors"`
}

package suppliers

import (
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type SupplierFilter struct {
	Kind                adminplusdomain.SupplierKind
	Type                adminplusdomain.SupplierType
	RuntimeStatus       adminplusdomain.SupplierRuntimeStatus
	HealthStatus        adminplusdomain.SupplierHealthStatus
	CapabilityStatus    adminplusdomain.SupplierCapabilityStatus
	IntegrationProtocol string
	PlatformHint        string
	PlatformFamily      string
	Query               string
}

func normalizeSupplierFilter(filter SupplierFilter) (SupplierFilter, error) {
	if filter.Kind != "" && !filter.Kind.Valid() {
		return SupplierFilter{}, badRequest("SUPPLIER_KIND_INVALID", "invalid supplier kind")
	}
	if filter.Type != "" && !filter.Type.Valid() {
		return SupplierFilter{}, badRequest("SUPPLIER_TYPE_INVALID", "invalid supplier type")
	}
	if filter.RuntimeStatus != "" && !filter.RuntimeStatus.Valid() {
		return SupplierFilter{}, badRequest("SUPPLIER_RUNTIME_STATUS_INVALID", "invalid supplier runtime status")
	}
	if filter.HealthStatus != "" && !filter.HealthStatus.Valid() {
		return SupplierFilter{}, badRequest("SUPPLIER_HEALTH_STATUS_INVALID", "invalid supplier health status")
	}
	if filter.CapabilityStatus != "" && !filter.CapabilityStatus.Valid() {
		return SupplierFilter{}, badRequest("SUPPLIER_CAPABILITY_STATUS_INVALID", "invalid supplier capability status")
	}
	filter.IntegrationProtocol = normalizeIntegrationProtocol(filter.IntegrationProtocol)
	if filter.IntegrationProtocol != "" && !validIntegrationProtocol(filter.IntegrationProtocol) {
		return SupplierFilter{}, badRequest("SUPPLIER_INTEGRATION_PROTOCOL_INVALID", "invalid supplier integration protocol")
	}
	filter.PlatformHint = normalizeSupplierDerivedFilterValue(filter.PlatformHint)
	filter.PlatformFamily = normalizeSupplierDerivedFilterValue(filter.PlatformFamily)
	if filter.PlatformFamily != "" && !validSupplierPlatformFamily(filter.PlatformFamily) {
		return SupplierFilter{}, badRequest("SUPPLIER_PLATFORM_FAMILY_INVALID", "invalid supplier platform family")
	}
	filter.Query = strings.ToLower(strings.TrimSpace(filter.Query))
	return filter, nil
}

func supplierMatchesCapabilityStatus(supplier *adminplusdomain.Supplier, status adminplusdomain.SupplierCapabilityStatus) bool {
	if status == "" {
		return true
	}
	if supplier == nil {
		return false
	}
	for _, capability := range supplier.Capabilities {
		if capability.Status == status {
			return true
		}
	}
	return false
}

func supplierMatchesIntegrationProtocol(supplier *adminplusdomain.Supplier, protocol string) bool {
	if protocol == "" {
		return true
	}
	if supplier == nil || supplier.IntegrationHint == nil {
		return false
	}
	return normalizeIntegrationProtocol(supplier.IntegrationHint.Protocol) == protocol
}

func supplierMatchesPlatformHint(supplier *adminplusdomain.Supplier, platformHint string) bool {
	if platformHint == "" {
		return true
	}
	if supplier == nil || supplier.PlatformHint == nil {
		return false
	}
	return normalizeSupplierDerivedFilterValue(supplier.PlatformHint.ID) == platformHint
}

func supplierMatchesPlatformFamily(supplier *adminplusdomain.Supplier, platformFamily string) bool {
	if platformFamily == "" {
		return true
	}
	if supplier == nil || supplier.PlatformHint == nil {
		return false
	}
	return normalizeSupplierDerivedFilterValue(supplier.PlatformHint.Family) == platformFamily
}

func normalizeIntegrationProtocol(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeSupplierDerivedFilterValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validIntegrationProtocol(value string) bool {
	switch value {
	case "openai", "claude", "gemini":
		return true
	default:
		return false
	}
}

func validSupplierPlatformFamily(value string) bool {
	switch value {
	case "sub2api", "new_api", "source_account", "api_provider", "cli_proxy", "browser_only":
		return true
	default:
		return false
	}
}

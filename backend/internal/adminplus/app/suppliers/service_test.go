package suppliers

import (
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func supplierCapabilityStatus(t *testing.T, supplier *adminplusdomain.Supplier, key string) adminplusdomain.SupplierCapabilityStatus {
	t.Helper()
	for _, capability := range supplier.Capabilities {
		if capability.Key == key {
			return capability.Status
		}
	}
	require.Failf(t, "missing supplier capability", "key=%s capabilities=%v", key, supplier.Capabilities)
	return ""
}

func supplierAPIEndpointCandidate(t *testing.T, supplier *adminplusdomain.Supplier, id string) adminplusdomain.SupplierAPIEndpointCandidate {
	t.Helper()
	for _, candidate := range supplier.APIEndpointCandidates {
		if candidate.ID == id {
			return candidate
		}
	}
	require.Failf(t, "missing supplier api endpoint candidate", "id=%s candidates=%v", id, supplier.APIEndpointCandidates)
	return adminplusdomain.SupplierAPIEndpointCandidate{}
}

func supplierOperationHint(t *testing.T, supplier *adminplusdomain.Supplier, key string) adminplusdomain.SupplierOperationHint {
	t.Helper()
	for _, hint := range supplier.OperationHints {
		if hint.Key == key {
			return hint
		}
	}
	require.Failf(t, "missing supplier operation hint", "key=%s hints=%v", key, supplier.OperationHints)
	return adminplusdomain.SupplierOperationHint{}
}

func supplierURLHintByKey(t *testing.T, supplier *adminplusdomain.Supplier, key string) adminplusdomain.SupplierURLHint {
	t.Helper()
	for _, hint := range supplier.URLHints {
		if hint.Key == key {
			return hint
		}
	}
	require.Failf(t, "missing supplier url hint", "key=%s hints=%v", key, supplier.URLHints)
	return adminplusdomain.SupplierURLHint{}
}

package ports

import (
	"context"
	"time"
)

var _ ProviderAdapter = (*stubProviderAdapter)(nil)

type stubProviderAdapter struct{}

func (s *stubProviderAdapter) Identity() ProviderIdentity {
	return ProviderIdentity{SupplierID: 1, Kind: ProviderKindSub2API}
}

func (s *stubProviderAdapter) FetchRateCatalog(_ context.Context, _ FetchContext) ([]ProviderRateEntry, error) {
	return nil, nil
}

func (s *stubProviderAdapter) FetchBalance(_ context.Context, _ FetchContext) (*ProviderBalanceSnapshotInput, error) {
	return &ProviderBalanceSnapshotInput{SupplierID: 1}, nil
}

func (s *stubProviderAdapter) FetchAnnouncements(_ context.Context, _ FetchContext) ([]ProviderAnnouncement, error) {
	return nil, nil
}

func (s *stubProviderAdapter) FetchHealthSample(_ context.Context, _ FetchContext) (*ProviderHealthSampleInput, error) {
	capturedAt := time.Now()
	return &ProviderHealthSampleInput{SupplierID: 1, CapturedAt: &capturedAt}, nil
}

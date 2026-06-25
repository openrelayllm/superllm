package sitediscovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	mailverificationapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/mailverification"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func TestParseDaheiAIItems(t *testing.T) {
	body := `
		<section id="third-party">
			<a class="card" href="https://example.com/register" data-site-id="site-1" data-domain="example.com">
				<div class="name">Example New API</div>
				<div class="desc">new-api 模板渠道</div>
			</a>
		</section>
	`
	items, err := parseDaheiAIItems(DefaultSourceURL, body)
	if err != nil {
		t.Fatalf("parse items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item.SourceSiteID != "site-1" {
		t.Fatalf("unexpected site id: %q", item.SourceSiteID)
	}
	if item.SourceSection != "third-party" {
		t.Fatalf("unexpected section: %q", item.SourceSection)
	}
	if item.RegisterURL != "https://example.com/register" {
		t.Fatalf("unexpected register url: %q", item.RegisterURL)
	}
	if item.APIBaseURL != "https://example.com" {
		t.Fatalf("unexpected api base url: %q", item.APIBaseURL)
	}
}

func TestClassifyItem(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected adminplusdomain.SupplierType
	}{
		{name: "new api", text: "new-api 模板 支持 New-Api-User", expected: adminplusdomain.SupplierTypeNewAPI},
		{name: "sub2api", text: "sub2api admin channel", expected: adminplusdomain.SupplierTypeSub2API},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyItem(&adminplusdomain.SiteDiscoveryItem{
				Name:        tt.text,
				Description: tt.text,
			})
			if result.Status != adminplusdomain.SiteDiscoveryClassificationSupported {
				t.Fatalf("expected supported, got %s", result.Status)
			}
			if result.ProviderType != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, result.ProviderType)
			}
		})
	}

	unknown := classifyItem(&adminplusdomain.SiteDiscoveryItem{Name: "plain site"})
	if unknown.Status != adminplusdomain.SiteDiscoveryClassificationUnknown {
		t.Fatalf("expected unknown, got %s", unknown.Status)
	}
}

func TestProbeSiteClassificationKnownInterfaces(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		body     string
		expected adminplusdomain.SupplierType
	}{
		{
			name: "new api status",
			path: "/api/status",
			body: `{
				"success": true,
				"message": "",
				"data": {
					"version": "v0.10.0",
					"quota_per_unit": 500000,
					"system_name": "New API",
					"setup": false,
					"register_enabled": true
				}
			}`,
			expected: adminplusdomain.SupplierTypeNewAPI,
		},
		{
			name: "sub2api public settings",
			path: "/api/v1/settings/public",
			body: `{
				"code": 0,
				"message": "success",
				"data": {
					"version": "0.11.3",
					"site_name": "Sub2API",
					"api_base_url": "https://api.example.com",
					"registration_enabled": true,
					"table_default_page_size": 20
				}
			}`,
			expected: adminplusdomain.SupplierTypeSub2API,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.path {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			service := NewService(nil, nil, nil, nil, nil, server.Client())
			result := service.probeSiteClassification(context.Background(), &adminplusdomain.SiteDiscoveryItem{
				APIBaseURL: server.URL,
			})
			if result.Status != adminplusdomain.SiteDiscoveryClassificationSupported {
				t.Fatalf("expected supported, got %s", result.Status)
			}
			if result.ProviderType != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, result.ProviderType)
			}
			if result.Confidence < 0.95 {
				t.Fatalf("expected high confidence, got %.2f", result.Confidence)
			}
		})
	}
}

func TestClassifyCandidatesStreamYieldsFastItemsBeforeSlowItems(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer slowServer.Close()
	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/settings/public" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"data": {
				"version": "0.11.3",
				"site_name": "Sub2API",
				"api_base_url": "https://api.example.com",
				"registration_enabled": true,
				"table_default_page_size": 20
			}
		}`))
	}))
	defer fastServer.Close()

	service := NewService(nil, nil, nil, nil, nil, fastServer.Client())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := service.classifyCandidatesStream(ctx, []*adminplusdomain.SiteDiscoveryItem{
		{Name: "slow", APIBaseURL: slowServer.URL},
		{Name: "fast", APIBaseURL: fastServer.URL},
	}, true, false)

	select {
	case item := <-results:
		if item == nil {
			t.Fatal("expected first classified item")
		}
		if item.APIBaseURL != fastServer.URL {
			t.Fatalf("expected fast item first, got %s", item.APIBaseURL)
		}
		if item.ProviderType != adminplusdomain.SupplierTypeSub2API {
			t.Fatalf("expected fast item classified as sub2api, got %s", item.ProviderType)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected fast item before slow endpoint timeout")
	}
}

func TestProbeKnownProviderInterfacesTriesNewAPIWhenSub2APIEndpointIsSlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/settings/public":
			<-r.Context().Done()
		case "/api/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"version": "v0.10.0",
					"quota_per_unit": 500000,
					"system_name": "New API",
					"setup": false,
					"register_enabled": true
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(nil, nil, nil, nil, nil, server.Client())
	result := service.probeKnownProviderInterfaces(context.Background(), &adminplusdomain.SiteDiscoveryItem{
		APIBaseURL: server.URL,
	})
	if result.ProviderType != adminplusdomain.SupplierTypeNewAPI {
		t.Fatalf("expected new-api after slow sub2api endpoint, got %s", result.ProviderType)
	}
}

func TestClassifyCandidateDoesNotDowngradeExistingSupportedItem(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	service := NewService(nil, nil, nil, nil, nil, server.Client())
	item := &adminplusdomain.SiteDiscoveryItem{
		Name:                     "existing supported",
		APIBaseURL:               server.URL,
		ProviderType:             adminplusdomain.SupplierTypeNewAPI,
		ClassificationStatus:     adminplusdomain.SiteDiscoveryClassificationSupported,
		ClassificationConfidence: 0.98,
		ClassificationEvidence:   []string{"api:/api/status", "api:new_api_status"},
	}
	service.classifyCandidate(context.Background(), item, true, false)
	if item.ProviderType != adminplusdomain.SupplierTypeNewAPI {
		t.Fatalf("expected existing provider to be preserved, got %s", item.ProviderType)
	}
	if item.ClassificationStatus != adminplusdomain.SiteDiscoveryClassificationSupported {
		t.Fatalf("expected supported status to be preserved, got %s", item.ClassificationStatus)
	}
}

func TestClassifyCandidateNormalizesUnknownStatus(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil, nil)
	item := &adminplusdomain.SiteDiscoveryItem{Name: "plain relay"}
	service.classifyCandidate(context.Background(), item, false, false)
	if item.ClassificationStatus != adminplusdomain.SiteDiscoveryClassificationUnknown {
		t.Fatalf("expected unknown status, got %q", item.ClassificationStatus)
	}
}

func TestGenerateRegistrationPassword(t *testing.T) {
	password, err := generateRegistrationPassword(adminplusdomain.SupplierTypeNewAPI)
	if err != nil {
		t.Fatalf("generate password: %v", err)
	}
	if len(password) != defaultPasswordLength {
		t.Fatalf("expected length %d, got %d", defaultPasswordLength, len(password))
	}
	for _, chars := range []string{
		"abcdefghijkmnopqrstuvwxyz",
		"ABCDEFGHJKLMNPQRSTUVWXYZ",
		"23456789",
		"!@#_-",
	} {
		if !strings.ContainsAny(password, chars) {
			t.Fatalf("password %q does not contain a char from %q", password, chars)
		}
	}
}

func TestRegisterItemQueuesTaskWithoutCreatingSupplier(t *testing.T) {
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, nil, plaintextCredentialCipher{}, nil)

	credential, task, err := service.RegisterItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	if credential.SupplierID != 0 {
		t.Fatalf("expected registration credential without supplier before success, got %d", credential.SupplierID)
	}
	if task.SupplierID != 0 {
		t.Fatalf("expected registration task without supplier before success, got %d", task.SupplierID)
	}
	suppliers, err := supplierService.List(context.Background(), suppliersapp.SupplierFilter{})
	if err != nil {
		t.Fatalf("list suppliers: %v", err)
	}
	if len(suppliers) != 0 {
		t.Fatalf("expected no supplier before registration success, got %d", len(suppliers))
	}
}

func TestProcessRegistrationTaskResultCreatesSupplierWithCredential(t *testing.T) {
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, nil, plaintextCredentialCipher{}, nil)
	_, task, err := service.RegisterItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	processor := NewRegistrationProcessor(repo, supplierService, plaintextCredentialCipher{})

	ingest, err := processor.ProcessRegistrationTaskResult(context.Background(), task, map[string]any{
		"registration_submitted": true,
		"register_url":           item.RegisterURL,
		"provider_type":          string(item.ProviderType),
	})
	if err != nil {
		t.Fatalf("process registration: %v", err)
	}
	supplierID, _ := ingest["supplier_id"].(int64)
	if supplierID <= 0 {
		t.Fatalf("expected supplier id in ingest result, got %#v", ingest)
	}
	credential, _, err := repo.GetRegistrationCredentialByTaskID(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get credential: %v", err)
	}
	if credential.Status != adminplusdomain.SupplierRegistrationStatusSucceeded {
		t.Fatalf("expected succeeded credential, got %s", credential.Status)
	}
	if repo.items[item.ID].SupplierID != supplierID {
		t.Fatalf("expected item linked to supplier %d, got %d", supplierID, repo.items[item.ID].SupplierID)
	}
	browserCredential, err := supplierService.GetBrowserCredential(context.Background(), supplierID)
	if err != nil {
		t.Fatalf("get browser credential: %v", err)
	}
	if browserCredential.Username != "ops@example.com" {
		t.Fatalf("expected registration email persisted, got %q", browserCredential.Username)
	}
	if browserCredential.Password == "" {
		t.Fatal("expected generated password persisted")
	}
}

func TestProcessRegistrationTaskResultIncompleteFailsWithoutSupplier(t *testing.T) {
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, nil, plaintextCredentialCipher{}, nil)
	_, task, err := service.RegisterItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	processor := NewRegistrationProcessor(repo, supplierService, plaintextCredentialCipher{})

	ingest, err := processor.ProcessRegistrationTaskResult(context.Background(), task, map[string]any{
		"registration_submitted": false,
	})
	if err != nil {
		t.Fatalf("process registration: %v", err)
	}
	if ingest["registration_status"] != string(adminplusdomain.SupplierRegistrationStatusFailed) {
		t.Fatalf("expected failed registration ingest, got %#v", ingest)
	}
	credential, _, err := repo.GetRegistrationCredentialByTaskID(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get credential: %v", err)
	}
	if credential.Status != adminplusdomain.SupplierRegistrationStatusFailed {
		t.Fatalf("expected failed credential, got %s", credential.Status)
	}
	if credential.SupplierID != 0 {
		t.Fatalf("expected no supplier id for incomplete registration, got %d", credential.SupplierID)
	}
	if repo.items[item.ID].SupplierID != 0 {
		t.Fatalf("expected item not linked to supplier, got %d", repo.items[item.ID].SupplierID)
	}
}

func TestProcessRegistrationTaskFailureMarksManualVerificationWithoutSupplier(t *testing.T) {
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, nil, plaintextCredentialCipher{}, nil)
	_, task, err := service.RegisterItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	processor := NewRegistrationProcessor(repo, supplierService, plaintextCredentialCipher{})

	ingest, err := processor.ProcessRegistrationTaskFailure(context.Background(), task, "REGISTRATION_VERIFICATION_REQUIRED", "需要验证码")
	if err != nil {
		t.Fatalf("process failure: %v", err)
	}
	if ingest["registration_status"] != string(adminplusdomain.SupplierRegistrationStatusWaitingManualVerification) {
		t.Fatalf("expected manual verification ingest, got %#v", ingest)
	}
	credential, _, err := repo.GetRegistrationCredentialByTaskID(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get credential: %v", err)
	}
	if credential.SupplierID != 0 {
		t.Fatalf("expected no supplier id for manual verification, got %d", credential.SupplierID)
	}
	if repo.items[item.ID].SupplierID != 0 {
		t.Fatalf("expected item not linked to supplier, got %d", repo.items[item.ID].SupplierID)
	}
}

func TestReadTaskRegistrationVerificationCodeUsesLeasedTaskAndRegistrationEmail(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	mailReader := &fakeRegistrationMailReader{}
	service := NewService(repo, supplierService, extensionService, mailReader, plaintextCredentialCipher{}, nil)
	_, task, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	claimed, err := extensionService.ClaimTask(ctx, extensionapp.ClaimTaskInput{
		DeviceID: "device-1",
		Types:    []adminplusdomain.ExtensionTaskType{adminplusdomain.ExtensionTaskTypeRegisterSupplier},
		LeaseTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("claim task: %v", err)
	}
	heartbeat, err := extensionService.Heartbeat(ctx, extensionapp.HeartbeatInput{
		TaskID:     claimed.ID,
		DeviceID:   claimed.DeviceID,
		LeaseToken: claimed.LeaseToken,
		LeaseTTL:   time.Minute,
	})
	if err != nil {
		t.Fatalf("heartbeat task: %v", err)
	}

	result, err := service.ReadTaskRegistrationVerificationCode(ctx, ReadRegistrationVerificationCodeInput{
		TaskID:              task.ID,
		DeviceID:            heartbeat.DeviceID,
		LeaseToken:          heartbeat.LeaseToken,
		TriggeredAt:         ptrRegistrationTime(time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)),
		TimeoutSeconds:      2,
		PollIntervalSeconds: 2,
	})

	if err != nil {
		t.Fatalf("read verification code: %v", err)
	}
	if result.Code != "654321" {
		t.Fatalf("expected code 654321, got %q", result.Code)
	}
	if mailReader.lastInput.Email != "ops@example.com" {
		t.Fatalf("expected registration email, got %q", mailReader.lastInput.Email)
	}
	if mailReader.lastInput.ClaimKey != "registration_task:"+stringFromInt64(task.ID) {
		t.Fatalf("expected task claim key, got %q", mailReader.lastInput.ClaimKey)
	}
	if mailReader.lastInput.To != "ops@example.com" {
		t.Fatalf("expected Gmail recipient filter to use registration email, got %q", mailReader.lastInput.To)
	}
	if mailReader.lastInput.TriggeredAt == nil || !mailReader.lastInput.TriggeredAt.Equal(time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected triggered_at to pass through, got %#v", mailReader.lastInput.TriggeredAt)
	}
}

func TestReadTaskRegistrationVerificationCodeRejectsWrongLease(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, &fakeRegistrationMailReader{}, plaintextCredentialCipher{}, nil)
	_, task, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}

	result, err := service.ReadTaskRegistrationVerificationCode(ctx, ReadRegistrationVerificationCodeInput{
		TaskID:     task.ID,
		DeviceID:   "device-1",
		LeaseToken: "wrong-token",
	})

	if result != nil {
		t.Fatalf("expected no result with wrong lease, got %#v", result)
	}
	if infraerrors.Reason(err) != "EXTENSION_TASK_LEASE_MISMATCH" {
		t.Fatalf("expected lease mismatch, got %v", err)
	}
}

type fakeRegistrationMailReader struct {
	lastInput mailverificationapp.ReadVerificationCodeForEmailInput
}

func (r *fakeRegistrationMailReader) ReadVerificationCodeForEmail(_ context.Context, in mailverificationapp.ReadVerificationCodeForEmailInput) (*mailverificationapp.ReadVerificationCodeResult, error) {
	r.lastInput = in
	return &mailverificationapp.ReadVerificationCodeResult{
		Provider:   mailverificationapp.ProviderGmail,
		Code:       "654321",
		MessageID:  "message-1",
		ReceivedAt: time.Date(2026, 6, 25, 12, 0, 30, 0, time.UTC),
	}, nil
}

func ptrRegistrationTime(value time.Time) *time.Time {
	return &value
}

type plaintextCredentialCipher struct{}

func (plaintextCredentialCipher) Encrypt(plaintext string) (string, error) {
	return plaintext, nil
}

func (plaintextCredentialCipher) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

type registrationMemoryRepository struct {
	mu          sync.Mutex
	settings    adminplusdomain.SiteDiscoverySettings
	nextItemID  int64
	nextCredID  int64
	items       map[int64]*adminplusdomain.SiteDiscoveryItem
	credentials map[int64]*adminplusdomain.SupplierRegistrationCredential
}

func newRegistrationMemoryRepository() *registrationMemoryRepository {
	return &registrationMemoryRepository{
		settings: adminplusdomain.SiteDiscoverySettings{
			RegistrationEmail:   "ops@example.com",
			RegistrationEnabled: true,
			LowRateThreshold:    defaultLowRateThreshold,
			UpdatedAt:           time.Now().UTC(),
		},
		nextItemID:  1,
		nextCredID:  1,
		items:       make(map[int64]*adminplusdomain.SiteDiscoveryItem),
		credentials: make(map[int64]*adminplusdomain.SupplierRegistrationCredential),
	}
}

func (r *registrationMemoryRepository) addSupportedItem() *adminplusdomain.SiteDiscoveryItem {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := &adminplusdomain.SiteDiscoveryItem{
		ID:                       r.nextItemID,
		RunID:                    1,
		SourceURL:                DefaultSourceURL,
		SourceSiteID:             "site-1",
		SourceSection:            "third-party",
		Name:                     "Example New API",
		RegisterURL:              "https://example.com/register",
		DashboardURL:             "https://example.com",
		APIBaseURL:               "https://example.com",
		Host:                     "example.com",
		ProviderType:             adminplusdomain.SupplierTypeNewAPI,
		ClassificationStatus:     adminplusdomain.SiteDiscoveryClassificationSupported,
		ClassificationConfidence: 0.98,
		ImportStatus:             adminplusdomain.SiteDiscoveryImportNew,
		ProcessStatus:            adminplusdomain.SiteDiscoveryProcessUnprocessed,
		CreatedAt:                time.Now().UTC(),
		UpdatedAt:                time.Now().UTC(),
	}
	r.nextItemID++
	r.items[item.ID] = cloneRegistrationItem(item)
	return cloneRegistrationItem(item)
}

func (r *registrationMemoryRepository) GetSettings(context.Context) (*adminplusdomain.SiteDiscoverySettings, error) {
	settings := r.settings
	return &settings, nil
}

func (r *registrationMemoryRepository) UpdateSettings(_ context.Context, settings adminplusdomain.SiteDiscoverySettings) (*adminplusdomain.SiteDiscoverySettings, error) {
	r.settings = settings
	return &settings, nil
}

func (r *registrationMemoryRepository) CreateRun(context.Context, *adminplusdomain.SiteDiscoveryRun) (*adminplusdomain.SiteDiscoveryRun, error) {
	panic("not implemented")
}

func (r *registrationMemoryRepository) UpdateRun(context.Context, *adminplusdomain.SiteDiscoveryRun) (*adminplusdomain.SiteDiscoveryRun, error) {
	panic("not implemented")
}

func (r *registrationMemoryRepository) FindExistingItem(context.Context, string, string, string) (*adminplusdomain.SiteDiscoveryItem, error) {
	return nil, nil
}

func (r *registrationMemoryRepository) UpsertItem(_ context.Context, item *adminplusdomain.SiteDiscoveryItem) (*adminplusdomain.SiteDiscoveryItem, error) {
	return item, nil
}

func (r *registrationMemoryRepository) GetItem(_ context.Context, id int64) (*adminplusdomain.SiteDiscoveryItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneRegistrationItem(r.items[id]), nil
}

func (r *registrationMemoryRepository) ListItems(context.Context, ListFilter) ([]*adminplusdomain.SiteDiscoveryItem, error) {
	return nil, nil
}

func (r *registrationMemoryRepository) LinkSupplier(_ context.Context, itemID int64, supplierID int64) (*adminplusdomain.SiteDiscoveryItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := cloneRegistrationItem(r.items[itemID])
	item.SupplierID = supplierID
	item.ImportStatus = adminplusdomain.SiteDiscoveryImportImported
	item.ProcessStatus = adminplusdomain.SiteDiscoveryProcessRegistered
	r.items[itemID] = item
	return cloneRegistrationItem(item), nil
}

func (r *registrationMemoryRepository) UpsertRegistrationCredential(_ context.Context, credential *adminplusdomain.SupplierRegistrationCredential) (*adminplusdomain.SupplierRegistrationCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := cloneRegistrationCredential(credential)
	for _, existing := range r.credentials {
		if existing.DiscoveryID == cp.DiscoveryID {
			cp.ID = existing.ID
			cp.CreatedAt = existing.CreatedAt
			r.credentials[cp.ID] = cp
			return cloneRegistrationCredential(cp), nil
		}
	}
	cp.ID = r.nextCredID
	r.nextCredID++
	r.credentials[cp.ID] = cp
	return cloneRegistrationCredential(cp), nil
}

func (r *registrationMemoryRepository) UpdateRegistrationTask(_ context.Context, credentialID int64, taskID int64, status adminplusdomain.SupplierRegistrationStatus, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := cloneRegistrationCredential(r.credentials[credentialID])
	cp.ExtensionTaskID = taskID
	cp.Status = status
	cp.LastAttemptAt = &attemptedAt
	r.credentials[credentialID] = cp
	return cloneRegistrationCredential(cp), nil
}

func (r *registrationMemoryRepository) GetRegistrationCredentialByTaskID(_ context.Context, taskID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.SiteDiscoveryItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, credential := range r.credentials {
		if credential.ExtensionTaskID == taskID {
			return cloneRegistrationCredential(credential), cloneRegistrationItem(r.items[credential.DiscoveryID]), nil
		}
	}
	return nil, nil, nil
}

func (r *registrationMemoryRepository) CompleteRegistration(_ context.Context, credentialID int64, supplierID int64, status adminplusdomain.SupplierRegistrationStatus, errorCode string, errorMessage string, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := cloneRegistrationCredential(r.credentials[credentialID])
	if supplierID > 0 {
		cp.SupplierID = supplierID
	}
	cp.Status = status
	cp.ErrorCode = errorCode
	cp.ErrorMessage = errorMessage
	cp.LastAttemptAt = &attemptedAt
	r.credentials[credentialID] = cp
	return cloneRegistrationCredential(cp), nil
}

func (r *registrationMemoryRepository) ListRecommendations(context.Context, float64, int) ([]*adminplusdomain.SiteDiscoveryRecommendation, error) {
	return nil, nil
}

func cloneRegistrationItem(in *adminplusdomain.SiteDiscoveryItem) *adminplusdomain.SiteDiscoveryItem {
	if in == nil {
		return nil
	}
	cp := *in
	return &cp
}

func cloneRegistrationCredential(in *adminplusdomain.SupplierRegistrationCredential) *adminplusdomain.SupplierRegistrationCredential {
	if in == nil {
		return nil
	}
	cp := *in
	return &cp
}

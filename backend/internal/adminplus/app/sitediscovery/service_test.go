package sitediscovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	suppliersapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/suppliers"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	opsservice "github.com/Wei-Shaw/sub2api/internal/service"
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

func TestParseSourceCandidatesUsesKanLLMSummary(t *testing.T) {
	body := `{
		"generatedAt": "2026-06-26T13:52:47Z",
		"apis": [
			{
				"id": "api-1",
				"name": "5YuanToken",
				"websiteUrl": "https://5yuantoken.org",
				"planType": "GPT Pro",
				"isSelfPurchased": true,
				"priceMultiplier": 0.16,
				"enabled": true,
				"available": true,
				"successRates": {"24h": 0.95},
				"checkedAt": "2026-06-26T13:49:01Z"
			},
			{
				"id": "api-2",
				"name": "5YuanToken",
				"websiteUrl": "https://5yuantoken.org/",
				"planType": "GPT Plus",
				"priceMultiplier": 0.10,
				"enabled": true,
				"available": false,
				"errorType": "http_error",
				"errorMessage": "503 Service Unavailable"
			},
			{
				"id": "api-3",
				"name": "937",
				"websiteUrl": "https://sub2api.937auth.vip",
				"planType": "GPT Mixed",
				"priceMultiplier": 0.20,
				"enabled": true,
				"available": true
			}
		]
	}`
	service := NewService(nil, nil, nil, nil, nil)
	items, err := service.parseSourceCandidates(context.Background(), nil, "https://www.kanllm.com/", body)
	if err != nil {
		t.Fatalf("parse source candidates: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 aggregated candidates, got %d", len(items))
	}
	first := items[0]
	if first.SourceSection != "kanllm" {
		t.Fatalf("expected kanllm section, got %q", first.SourceSection)
	}
	if first.SourceSiteID != "5yuantoken.org" {
		t.Fatalf("unexpected source site id: %q", first.SourceSiteID)
	}
	if first.RegisterURL != "https://5yuantoken.org" {
		t.Fatalf("unexpected register url: %q", first.RegisterURL)
	}
	if !strings.Contains(first.Description, "最低倍率 0.1") {
		t.Fatalf("expected min rate in description, got %q", first.Description)
	}
	if first.RawPayload["total_count"] != 2 {
		t.Fatalf("expected aggregated total count, got %#v", first.RawPayload["total_count"])
	}
}

func TestParseSourceCandidatesFallsBackToDirectSite(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	service := NewService(nil, nil, nil, nil, server.Client())
	items, err := service.parseSourceCandidates(context.Background(), server.Client(), server.URL, `<!doctype html><title>Plain Relay</title><div id="app"></div>`)
	if err != nil {
		t.Fatalf("parse source candidates: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 direct candidate, got %d", len(items))
	}
	item := items[0]
	if item.SourceSection != "direct-url" {
		t.Fatalf("expected direct-url section, got %q", item.SourceSection)
	}
	if item.Name != "Plain Relay" {
		t.Fatalf("expected title as name, got %q", item.Name)
	}
	if item.RegisterURL != server.URL {
		t.Fatalf("unexpected register url: %q", item.RegisterURL)
	}
	if item.Host == "" {
		t.Fatalf("unexpected host: %q", item.Host)
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
		{name: "subapi", text: "subapi admin channel", expected: adminplusdomain.SupplierTypeSub2API},
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

			service := NewService(nil, nil, nil, nil, server.Client())
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

	service := NewService(nil, nil, nil, nil, fastServer.Client())
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

	service := NewService(nil, nil, nil, nil, server.Client())
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

	service := NewService(nil, nil, nil, nil, server.Client())
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
	service := NewService(nil, nil, nil, nil, nil)
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

func TestRegisterItemRunsDirectRegistrationWithoutBrowserTask(t *testing.T) {
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	adapter := &fakeDirectRegistrationAdapter{
		results: []*ports.DirectRegistrationResult{{
			ProviderType: adminplusdomain.SupplierTypeNewAPI,
			Stage:        ports.DirectRegistrationStageCompleted,
			Submitted:    true,
			CapturedAt:   time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
		}},
	}
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(adapter)

	credential, task, err := service.RegisterItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	if task != nil {
		t.Fatalf("expected direct registration without browser task, got %#v", task)
	}
	if credential.SupplierID <= 0 {
		t.Fatalf("expected direct registration to import supplier, got %d", credential.SupplierID)
	}
	if credential.Status != adminplusdomain.SupplierRegistrationStatusSucceeded {
		t.Fatalf("expected succeeded workflow, got %s", credential.Status)
	}
	if len(adapter.inputs) != 1 {
		t.Fatalf("expected one direct registration call, got %d", len(adapter.inputs))
	}
	suppliers, err := supplierService.List(context.Background(), suppliersapp.SupplierFilter{})
	if err != nil {
		t.Fatalf("list suppliers: %v", err)
	}
	if len(suppliers) != 1 {
		t.Fatalf("expected supplier created after direct registration, got %d", len(suppliers))
	}
	browserCredential, err := supplierService.GetBrowserCredential(context.Background(), credential.SupplierID)
	if err != nil {
		t.Fatalf("get browser credential: %v", err)
	}
	if browserCredential.Username != "ops@example.com" {
		t.Fatalf("expected registration email as browser login username, got %q", browserCredential.Username)
	}
}

func TestRegisterItemQueuesBrowserFallbackOnlyWhenDirectRegistrationRequiresIt(t *testing.T) {
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	adapter := &fakeDirectRegistrationAdapter{
		errs: []error{infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "browser required")},
	}
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(adapter)

	firstCredential, firstTask, err := service.RegisterItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	if firstTask == nil {
		t.Fatal("expected browser fallback task")
	}
	secondCredential, secondTask, err := service.RegisterItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("register item again: %v", err)
	}

	if secondCredential.ID != firstCredential.ID {
		t.Fatalf("expected same credential, got %d and %d", firstCredential.ID, secondCredential.ID)
	}
	if secondTask.ID != firstTask.ID {
		t.Fatalf("expected same task, got %d and %d", firstTask.ID, secondTask.ID)
	}
	if len(adapter.inputs) != 1 {
		t.Fatalf("expected no duplicate direct registration call for active workflow, got %d", len(adapter.inputs))
	}
	tasks, err := extensionService.ListTasks(context.Background(), extensionapp.TaskFilter{Type: adminplusdomain.ExtensionTaskTypeRegisterSupplier})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected one active registration task, got %d", len(tasks))
	}
}

func TestListRegistrationTasksUsesExtensionTaskStatus(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
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
	if claimed.ID != task.ID {
		t.Fatalf("expected claimed task %d, got %d", task.ID, claimed.ID)
	}

	tasks, err := service.ListRegistrationTasks(ctx, ListFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list registration tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected one registration task, got %d", len(tasks))
	}
	if tasks[0].Status != adminplusdomain.SupplierRegistrationStatusRunning {
		t.Fatalf("expected running derived status, got %s", tasks[0].Status)
	}
	if tasks[0].TaskStatus != adminplusdomain.ExtensionTaskStatusClaimed {
		t.Fatalf("expected claimed task status, got %s", tasks[0].TaskStatus)
	}
	if tasks[0].Discovery == nil || tasks[0].Discovery.ID != item.ID {
		t.Fatalf("expected discovery item in task view, got %#v", tasks[0].Discovery)
	}
}

func TestListRegistrationTasksShowsQueuedWorkflowAfterRegister(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())

	credential, task, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	tasks, err := service.ListRegistrationTasks(ctx, ListFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list registration tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected one registration workflow, got %d", len(tasks))
	}
	got := tasks[0]
	if got.ID != credential.ID || got.RegistrationID != credential.ID {
		t.Fatalf("expected workflow id %d, got id=%d registration_id=%d", credential.ID, got.ID, got.RegistrationID)
	}
	if got.TaskID != task.ID {
		t.Fatalf("expected attempt task id %d, got %d", task.ID, got.TaskID)
	}
	if got.Status != adminplusdomain.SupplierRegistrationStatusQueued {
		t.Fatalf("expected queued registration workflow, got %s", got.Status)
	}
	if got.Discovery == nil || got.Discovery.RegistrationStatus != adminplusdomain.SupplierRegistrationStatusQueued {
		t.Fatalf("expected queued discovery projection, got %#v", got.Discovery)
	}
}

func TestRerunRegistrationQueuesNewTaskAfterFailure(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
	_, task, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	processor := NewRegistrationProcessor(repo, supplierService, plaintextCredentialCipher{})
	if _, err := processor.ProcessRegistrationTaskFailure(ctx, task, "REGISTRATION_VERIFICATION_CODE_NOT_FOUND", "未读取到验证码"); err != nil {
		t.Fatalf("process registration failure: %v", err)
	}

	credential, _, err := repo.GetRegistrationCredentialByTaskID(ctx, task.ID)
	if err != nil {
		t.Fatalf("get registration credential: %v", err)
	}
	credential, nextTask, err := service.RerunRegistration(ctx, credential.ID)
	if err != nil {
		t.Fatalf("rerun registration workflow: %v", err)
	}
	if nextTask.ID == task.ID {
		t.Fatalf("expected new rerun attempt, got same task %d", nextTask.ID)
	}
	if credential.Status != adminplusdomain.SupplierRegistrationStatusQueued {
		t.Fatalf("expected queued rerun credential, got %s", credential.Status)
	}
	if credential.ExtensionTaskID != nextTask.ID {
		t.Fatalf("expected credential linked to rerun task %d, got %d", nextTask.ID, credential.ExtensionTaskID)
	}
}

func TestRerunDirectFailureClearsStaleBrowserTask(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
	_, task, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	processor := NewRegistrationProcessor(repo, supplierService, plaintextCredentialCipher{})
	if _, err := processor.ProcessRegistrationTaskFailure(ctx, task, "REGISTRATION_FORM_NOT_FOUND", "未找到可填写的注册表单"); err != nil {
		t.Fatalf("process registration failure: %v", err)
	}
	credential, _, err := repo.GetRegistrationCredentialByTaskID(ctx, task.ID)
	if err != nil {
		t.Fatalf("get registration credential: %v", err)
	}
	service = NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(&fakeDirectRegistrationAdapter{
		alwaysErr: infraerrors.New(http.StatusConflict, "REGISTRATION_DISABLED", "new api registration is disabled"),
	})

	credential, nextTask, err := service.RerunRegistration(ctx, credential.ID)
	if err == nil {
		t.Fatal("expected direct registration failure")
	}
	if nextTask != nil {
		t.Fatalf("expected no browser fallback task, got %#v", nextTask)
	}
	if credential.ExtensionTaskID != 0 {
		t.Fatalf("expected stale extension task cleared, got %d", credential.ExtensionTaskID)
	}
	if credential.ErrorCode != "REGISTRATION_DISABLED" {
		t.Fatalf("expected direct failure reason, got %q", credential.ErrorCode)
	}
	records, err := service.ListRegistrationTasks(ctx, ListFilter{RegistrationStatus: adminplusdomain.SupplierRegistrationStatusFailed})
	if err != nil {
		t.Fatalf("list registration tasks: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one failed registration record, got %d", len(records))
	}
	if records[0].ErrorCode != "REGISTRATION_DISABLED" {
		t.Fatalf("expected direct failure in view, got %q", records[0].ErrorCode)
	}
	if records[0].TaskID != 0 {
		t.Fatalf("expected stale task omitted from view, got %d", records[0].TaskID)
	}
}

func TestListRegistrationTasksPrefersRegisteredDiscoveryOverStaleFailure(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
	_, task, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	processor := NewRegistrationProcessor(repo, supplierService, plaintextCredentialCipher{})
	if _, err := processor.ProcessRegistrationTaskFailure(ctx, task, "MAIL_VERIFICATION_CODE_NOT_FOUND", "未读取到验证码"); err != nil {
		t.Fatalf("process registration failure: %v", err)
	}
	if _, err := repo.LinkSupplier(ctx, item.ID, 36); err != nil {
		t.Fatalf("link supplier: %v", err)
	}

	records, err := service.ListRegistrationTasks(ctx, ListFilter{RegistrationStatus: adminplusdomain.SupplierRegistrationStatusSucceeded, Limit: 20})
	if err != nil {
		t.Fatalf("list succeeded registration tasks: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one succeeded registration record, got %d", len(records))
	}
	record := records[0]
	if record.Status != adminplusdomain.SupplierRegistrationStatusSucceeded {
		t.Fatalf("expected succeeded status, got %s", record.Status)
	}
	if record.TaskID != 0 || record.ErrorCode != "" || record.ErrorMessage != "" || record.CanRetry {
		t.Fatalf("expected stale task and error hidden from current view, got task=%d error=%q retry=%v", record.TaskID, record.ErrorCode, record.CanRetry)
	}
	if record.Discovery == nil || record.Discovery.SupplierID != 36 || record.Discovery.RegistrationStatus != adminplusdomain.SupplierRegistrationStatusSucceeded {
		t.Fatalf("expected registered discovery projection, got %#v", record.Discovery)
	}

	failedRecords, err := service.ListRegistrationTasks(ctx, ListFilter{RegistrationStatus: adminplusdomain.SupplierRegistrationStatusFailed, Limit: 20})
	if err != nil {
		t.Fatalf("list failed registration tasks: %v", err)
	}
	if len(failedRecords) != 0 {
		t.Fatalf("expected no failed current registration records, got %d", len(failedRecords))
	}
}

func TestRerunRegisteredDiscoveryDoesNotStartRegistrationWorkflow(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
	_, task, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	processor := NewRegistrationProcessor(repo, supplierService, plaintextCredentialCipher{})
	if _, err := processor.ProcessRegistrationTaskFailure(ctx, task, "REGISTRATION_FORM_NOT_FOUND", "未找到可填写的注册表单"); err != nil {
		t.Fatalf("process registration failure: %v", err)
	}
	if _, err := repo.LinkSupplier(ctx, item.ID, 36); err != nil {
		t.Fatalf("link supplier: %v", err)
	}
	credential, _, err := repo.GetRegistrationCredentialByTaskID(ctx, task.ID)
	if err != nil {
		t.Fatalf("get registration credential: %v", err)
	}
	direct := &fakeDirectRegistrationAdapter{
		alwaysErr: infraerrors.New(http.StatusConflict, "SHOULD_NOT_REGISTER", "direct registration should not be called"),
	}
	service = NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(direct)

	updated, nextTask, err := service.RerunRegistration(ctx, credential.ID)
	if err != nil {
		t.Fatalf("rerun registered discovery: %v", err)
	}
	if nextTask != nil {
		t.Fatalf("expected no registration task for already registered discovery, got %#v", nextTask)
	}
	if len(direct.inputs) != 0 {
		t.Fatalf("expected direct registration not called, got %d calls", len(direct.inputs))
	}
	if updated.Status != adminplusdomain.SupplierRegistrationStatusSucceeded || updated.ExtensionTaskID != 0 || updated.ErrorCode != "" {
		t.Fatalf("expected normalized succeeded credential, got %#v", updated)
	}
}

func TestSafeRegistrationErrorMessageUsesApplicationMessage(t *testing.T) {
	err := infraerrors.New(http.StatusConflict, "REGISTRATION_DISABLED", "new api registration is disabled")
	message := safeRegistrationErrorMessage(err)
	if message != "new api registration is disabled" {
		t.Fatalf("expected clean application message, got %q", message)
	}
	if strings.Contains(message, "metadata=map") || strings.Contains(message, "reason=") {
		t.Fatalf("expected message without internal error formatting, got %q", message)
	}
}

func TestRerunRegistrationCancelsRunningAttemptAndKeepsWorkflow(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
	credential, task, err := service.RegisterItem(ctx, item.ID)
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
	if _, err := extensionService.Heartbeat(ctx, extensionapp.HeartbeatInput{
		TaskID:     claimed.ID,
		DeviceID:   claimed.DeviceID,
		LeaseToken: claimed.LeaseToken,
		LeaseTTL:   time.Minute,
	}); err != nil {
		t.Fatalf("heartbeat task: %v", err)
	}

	nextCredential, nextTask, err := service.RerunRegistration(ctx, credential.ID)
	if err != nil {
		t.Fatalf("rerun running registration workflow: %v", err)
	}
	if nextCredential.ID != credential.ID {
		t.Fatalf("expected same registration workflow %d, got %d", credential.ID, nextCredential.ID)
	}
	if nextTask.ID == task.ID {
		t.Fatalf("expected new extension attempt, got same task %d", nextTask.ID)
	}
	oldTask, err := extensionService.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get old task: %v", err)
	}
	if oldTask.Status != adminplusdomain.ExtensionTaskStatusCancelled {
		t.Fatalf("expected old attempt cancelled, got %s", oldTask.Status)
	}
	if nextCredential.ExtensionTaskID != nextTask.ID {
		t.Fatalf("expected credential linked to new task %d, got %d", nextTask.ID, nextCredential.ExtensionTaskID)
	}
}

func TestRegisterItemRecordsRegistrationWorkflowLogWithoutSecrets(t *testing.T) {
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	writer := &registrationLogWriter{}
	service := NewService(
		repo,
		suppliersapp.NewService(suppliersapp.NewMemoryRepository()),
		extensionapp.NewService(extensionapp.NewMemoryRepository()),
		plaintextCredentialCipher{},
		nil,
	).WithDirectRegistration(browserFallbackRegistrationAdapter()).WithDiagnostics(bizlogs.NewRecorder(writer))

	credential, task, err := service.RegisterItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	if len(writer.inputs) < 2 {
		t.Fatalf("expected registration logs, got %d", len(writer.inputs))
	}
	seenFallback := false
	for _, input := range writer.inputs {
		if input.Component != "admin_plus.registration" {
			t.Fatalf("expected registration component, got %s", input.Component)
		}
		if strings.Contains(input.ExtraJSON, credential.Email) {
			t.Fatalf("registration log must not contain raw email: %s", input.ExtraJSON)
		}
		var extra map[string]any
		if err := json.Unmarshal([]byte(input.ExtraJSON), &extra); err != nil {
			t.Fatalf("parse log extra: %v", err)
		}
		registrationID, _ := extra["registration_id"].(float64)
		if int64(registrationID) != credential.ID {
			t.Fatalf("expected registration id %d in log, got %#v", credential.ID, extra["registration_id"])
		}
		if extra["action"] == "direct_registration_browser_fallback" {
			seenFallback = true
			taskID, _ := extra["task_id"].(float64)
			if int64(taskID) != task.ID {
				t.Fatalf("expected task id %d in fallback log, got %#v", task.ID, extra["task_id"])
			}
		}
	}
	if !seenFallback {
		t.Fatalf("expected browser fallback registration log, got %#v", writer.inputs)
	}
}

func TestRegisterItemFailureLogIncludesRequestDiagnostics(t *testing.T) {
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	writer := &registrationLogWriter{}
	service := NewService(
		repo,
		suppliersapp.NewService(suppliersapp.NewMemoryRepository()),
		extensionapp.NewService(extensionapp.NewMemoryRepository()),
		plaintextCredentialCipher{},
		nil,
	).WithDirectRegistration(&fakeDirectRegistrationAdapter{
		alwaysErr: infraerrors.New(http.StatusBadGateway, "SUPPLIER_DIRECT_REGISTRATION_FAILED", "new api registration endpoint is unreachable").WithMetadata(map[string]string{
			"endpoint":     "https://example.test/api/user/register",
			"error_kind":   "dns",
			"error_detail": "lookup example.test: no such host",
		}),
	}).WithDiagnostics(bizlogs.NewRecorder(writer))

	credential, task, err := service.RegisterItem(context.Background(), item.ID)
	if err == nil {
		t.Fatal("expected direct registration failure")
	}
	if task != nil {
		t.Fatalf("expected no browser fallback task, got %#v", task)
	}
	if credential == nil {
		t.Fatal("expected failed credential")
	}
	var failure map[string]any
	for _, input := range writer.inputs {
		var extra map[string]any
		if err := json.Unmarshal([]byte(input.ExtraJSON), &extra); err != nil {
			t.Fatalf("parse log extra: %v", err)
		}
		if extra["action"] == "direct_registration_failed" {
			failure = extra
			if strings.Contains(input.ExtraJSON, credential.Email) {
				t.Fatalf("registration log must not contain raw email: %s", input.ExtraJSON)
			}
			break
		}
	}
	if failure == nil {
		t.Fatalf("expected direct registration failure log, got %#v", writer.inputs)
	}
	if stringFromAny(failure["endpoint"]) != "https://example.test/api/user/register" {
		t.Fatalf("expected endpoint diagnostic, got %#v", failure["endpoint"])
	}
	if stringFromAny(failure["error_kind"]) != "dns" {
		t.Fatalf("expected dns error kind, got %#v", failure["error_kind"])
	}
	if stringFromAny(failure["error_detail"]) != "lookup example.test: no such host" {
		t.Fatalf("expected error detail, got %#v", failure["error_detail"])
	}
	if stringFromAny(failure["reason"]) != "SUPPLIER_DIRECT_REGISTRATION_FAILED" {
		t.Fatalf("expected failure reason, got %#v", failure["reason"])
	}
}

func TestListRegistrationLogsUsesWorkflowID(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	logReader := &registrationLogReader{
		logs: []*opsservice.OpsSystemLog{
			registrationSystemLog(1, "admin_plus.registration", "queued", map[string]any{"registration_id": "1", "task_id": "10"}),
			registrationSystemLog(2, "admin_plus.extension", "old attempt cancelled", map[string]any{"registration_id": "1", "task_id": "10"}),
			registrationSystemLog(3, "admin_plus.extension", "new attempt running", map[string]any{"registration_id": "1", "task_id": "11"}),
			registrationSystemLog(5, "admin_plus.registration", "other workflow", map[string]any{"registration_id": "2", "task_id": "20"}),
		},
	}
	service := NewService(
		repo,
		suppliersapp.NewService(suppliersapp.NewMemoryRepository()),
		extensionService,
		plaintextCredentialCipher{},
		nil,
	).WithDirectRegistration(browserFallbackRegistrationAdapter()).WithRegistrationLogs(logReader)

	credential, _, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	result, err := service.ListRegistrationLogs(ctx, credential.ID, 20)
	if err != nil {
		t.Fatalf("list registration logs: %v", err)
	}
	if len(result.Items) != 4 {
		t.Fatalf("expected workflow logs only, got %d: %#v", len(result.Items), result.Items)
	}
	seen := map[string]bool{}
	for _, log := range result.Items {
		seen[log.Message] = true
		if log.Message == "other workflow" {
			t.Fatalf("unexpected log from another registration workflow")
		}
	}
	for _, message := range []string{"queued", "old attempt cancelled", "new attempt running", "注册流程当前状态"} {
		if !seen[message] {
			t.Fatalf("expected log message %q in workflow logs, got %#v", message, seen)
		}
	}
}

func TestListRegistrationLogsReturnsCurrentStatusWithoutLogReader(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(
		repo,
		suppliersapp.NewService(suppliersapp.NewMemoryRepository()),
		extensionService,
		plaintextCredentialCipher{},
		nil,
	).WithDirectRegistration(browserFallbackRegistrationAdapter())
	credential, task, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}

	result, err := service.ListRegistrationLogs(ctx, credential.ID, 20)
	if err != nil {
		t.Fatalf("list registration logs: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one current status log, got %d", len(result.Items))
	}
	log := result.Items[0]
	if log.Message != "注册流程当前状态" {
		t.Fatalf("expected current status log, got %q", log.Message)
	}
	if stringFromAny(log.Extra["registration_id"]) != stringFromInt64(credential.ID) {
		t.Fatalf("expected registration id %d in snapshot, got %#v", credential.ID, log.Extra["registration_id"])
	}
	if stringFromAny(log.Extra["task_id"]) != stringFromInt64(task.ID) {
		t.Fatalf("expected task id %d in snapshot, got %#v", task.ID, log.Extra["task_id"])
	}
	if stringFromAny(log.Extra["outcome"]) != "" {
		t.Fatalf("expected active snapshot without outcome, got %#v", log.Extra["outcome"])
	}
}

func TestListRegistrationLogsSnapshotUsesCurrentRegistrationOutcome(t *testing.T) {
	ctx := context.Background()
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
	_, task, err := service.RegisterItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("register item: %v", err)
	}
	processor := NewRegistrationProcessor(repo, supplierService, plaintextCredentialCipher{})
	if _, err := processor.ProcessRegistrationTaskFailure(ctx, task, "REGISTRATION_DISABLED", "new api registration is disabled"); err != nil {
		t.Fatalf("process registration failure: %v", err)
	}
	credential, _, err := repo.GetRegistrationCredentialByTaskID(ctx, task.ID)
	if err != nil {
		t.Fatalf("get credential: %v", err)
	}

	result, err := service.ListRegistrationLogs(ctx, credential.ID, 20)
	if err != nil {
		t.Fatalf("list registration logs: %v", err)
	}
	var snapshot *opsservice.OpsSystemLog
	for _, item := range result.Items {
		if item.Message == "注册流程当前状态" {
			snapshot = item
			break
		}
	}
	if snapshot == nil {
		t.Fatalf("expected current status snapshot, got %#v", result.Items)
	}
	if snapshot.Level != bizlogs.LevelWarn {
		t.Fatalf("expected warn snapshot level, got %s", snapshot.Level)
	}
	if stringFromAny(snapshot.Extra["outcome"]) != bizlogs.OutcomeFailed {
		t.Fatalf("expected failed snapshot outcome, got %#v", snapshot.Extra["outcome"])
	}
	if stringFromAny(snapshot.Extra["registration_status"]) != string(adminplusdomain.SupplierRegistrationStatusFailed) {
		t.Fatalf("expected failed registration status, got %#v", snapshot.Extra["registration_status"])
	}
	if stringFromAny(snapshot.Extra["reason"]) != "REGISTRATION_DISABLED" {
		t.Fatalf("expected failure reason, got %#v", snapshot.Extra["reason"])
	}
}

func TestProcessRegistrationTaskResultCreatesSupplierWithCredential(t *testing.T) {
	repo := newRegistrationMemoryRepository()
	item := repo.addSupportedItem()
	supplierService := suppliersapp.NewService(suppliersapp.NewMemoryRepository())
	extensionService := extensionapp.NewService(extensionapp.NewMemoryRepository())
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
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
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
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
	service := NewService(repo, supplierService, extensionService, plaintextCredentialCipher{}, nil).WithDirectRegistration(browserFallbackRegistrationAdapter())
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

type fakeDirectRegistrationAdapter struct {
	inputs    []ports.DirectRegistrationInput
	results   []*ports.DirectRegistrationResult
	errs      []error
	alwaysErr error
}

func browserFallbackRegistrationAdapter() *fakeDirectRegistrationAdapter {
	return &fakeDirectRegistrationAdapter{
		alwaysErr: infraerrors.New(http.StatusConflict, "BROWSER_FALLBACK_REQUIRED", "browser registration fallback required"),
	}
}

func (a *fakeDirectRegistrationAdapter) RegisterAccount(_ context.Context, in ports.DirectRegistrationInput) (*ports.DirectRegistrationResult, error) {
	a.inputs = append(a.inputs, in)
	index := len(a.inputs) - 1
	if a.alwaysErr != nil {
		return nil, a.alwaysErr
	}
	if index < len(a.errs) && a.errs[index] != nil {
		return nil, a.errs[index]
	}
	if index < len(a.results) && a.results[index] != nil {
		return a.results[index], nil
	}
	return &ports.DirectRegistrationResult{
		ProviderType: in.ProviderType,
		Stage:        ports.DirectRegistrationStageCompleted,
		Submitted:    true,
		CapturedAt:   time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
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

type registrationLogWriter struct {
	inputs []*opsservice.OpsInsertSystemLogInput
}

func (w *registrationLogWriter) BatchInsertSystemLogs(_ context.Context, inputs []*opsservice.OpsInsertSystemLogInput) (int64, error) {
	w.inputs = append(w.inputs, inputs...)
	return int64(len(inputs)), nil
}

type registrationLogReader struct {
	logs []*opsservice.OpsSystemLog
}

func (r *registrationLogReader) ListSystemLogs(_ context.Context, filter *opsservice.OpsSystemLogFilter) (*opsservice.OpsSystemLogList, error) {
	out := make([]*opsservice.OpsSystemLog, 0)
	for _, log := range r.logs {
		if log == nil || filter == nil {
			continue
		}
		if filter.Component != "" && log.Component != filter.Component {
			continue
		}
		if !logMatchesExtraEquals(log, filter.ExtraEquals) {
			continue
		}
		out = append(out, log)
	}
	return &opsservice.OpsSystemLogList{
		Logs:     out,
		Total:    len(out),
		Page:     firstPositiveInt(filter.Page, 1),
		PageSize: firstPositiveInt(filter.PageSize, len(out)),
	}, nil
}

func registrationSystemLog(id int64, component string, message string, extra map[string]any) *opsservice.OpsSystemLog {
	return &opsservice.OpsSystemLog{
		ID:        id,
		CreatedAt: time.Date(2026, 6, 25, 12, 0, int(id), 0, time.UTC),
		Level:     bizlogs.LevelInfo,
		Component: component,
		Message:   message,
		Extra:     extra,
	}
}

func logMatchesExtraEquals(log *opsservice.OpsSystemLog, expected map[string]string) bool {
	for key, value := range expected {
		if log == nil || log.Extra == nil {
			return false
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		if stringFromAny(log.Extra[key]) != value {
			return false
		}
	}
	return true
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
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
	return r.applyRegistrationLocked(cloneRegistrationItem(r.items[id])), nil
}

func (r *registrationMemoryRepository) ListItems(_ context.Context, filter ListFilter) ([]*adminplusdomain.SiteDiscoveryItem, error) {
	return r.listItems(filter, false), nil
}

func (r *registrationMemoryRepository) ListRegistrationRecords(_ context.Context, filter ListFilter) ([]*RegistrationRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]*RegistrationRecord, 0, len(r.credentials))
	for _, credential := range r.credentials {
		if credential == nil {
			continue
		}
		item := cloneRegistrationItem(r.items[credential.DiscoveryID])
		if item == nil {
			continue
		}
		if filter.ProviderType != "" && item.ProviderType != filter.ProviderType {
			continue
		}
		item = r.applyRegistrationLocked(item)
		if filter.RegistrationStatus != "" && item.RegistrationStatus != filter.RegistrationStatus {
			continue
		}
		records = append(records, &RegistrationRecord{
			Credential: cloneRegistrationCredential(credential),
			Item:       item,
		})
	}
	if filter.Limit > 0 && len(records) > filter.Limit {
		records = records[:filter.Limit]
	}
	return records, nil
}

func (r *registrationMemoryRepository) listItems(filter ListFilter, onlyRegistration bool) []*adminplusdomain.SiteDiscoveryItem {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*adminplusdomain.SiteDiscoveryItem, 0, len(r.items))
	for _, item := range r.items {
		cp := r.applyRegistrationLocked(cloneRegistrationItem(item))
		if onlyRegistration && cp.RegistrationStatus == "" {
			continue
		}
		if filter.ProviderType != "" && cp.ProviderType != filter.ProviderType {
			continue
		}
		if filter.RegistrationStatus != "" && cp.RegistrationStatus != filter.RegistrationStatus {
			continue
		}
		out = append(out, cp)
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out
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
	cp.ErrorCode = ""
	cp.ErrorMessage = ""
	cp.LastAttemptAt = &attemptedAt
	r.credentials[credentialID] = cp
	return cloneRegistrationCredential(cp), nil
}

func (r *registrationMemoryRepository) StartRegistrationAttempt(_ context.Context, credentialID int64, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := cloneRegistrationCredential(r.credentials[credentialID])
	cp.ExtensionTaskID = 0
	cp.Status = adminplusdomain.SupplierRegistrationStatusRunning
	cp.VerificationStatus = ""
	cp.ErrorCode = ""
	cp.ErrorMessage = ""
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

func (r *registrationMemoryRepository) GetRegistrationCredential(_ context.Context, credentialID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.SiteDiscoveryItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	credential := cloneRegistrationCredential(r.credentials[credentialID])
	if credential == nil {
		return nil, nil, nil
	}
	return credential, cloneRegistrationItem(r.items[credential.DiscoveryID]), nil
}

func (r *registrationMemoryRepository) GetRegistrationCredentialByDiscoveryID(_ context.Context, discoveryID int64) (*adminplusdomain.SupplierRegistrationCredential, *adminplusdomain.SiteDiscoveryItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, credential := range r.credentials {
		if credential.DiscoveryID == discoveryID {
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

func (r *registrationMemoryRepository) MarkRegistrationSucceeded(_ context.Context, credentialID int64, supplierID int64, attemptedAt time.Time) (*adminplusdomain.SupplierRegistrationCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := cloneRegistrationCredential(r.credentials[credentialID])
	if supplierID > 0 {
		cp.SupplierID = supplierID
	}
	cp.ExtensionTaskID = 0
	cp.Status = adminplusdomain.SupplierRegistrationStatusSucceeded
	cp.VerificationStatus = ""
	cp.ErrorCode = ""
	cp.ErrorMessage = ""
	cp.LastAttemptAt = &attemptedAt
	r.credentials[credentialID] = cp
	return cloneRegistrationCredential(cp), nil
}

func (r *registrationMemoryRepository) ListRecommendations(context.Context, float64, int) ([]*adminplusdomain.SiteDiscoveryRecommendation, error) {
	return nil, nil
}

func (r *registrationMemoryRepository) applyRegistrationLocked(item *adminplusdomain.SiteDiscoveryItem) *adminplusdomain.SiteDiscoveryItem {
	if item == nil {
		return nil
	}
	for _, credential := range r.credentials {
		if credential.DiscoveryID != item.ID {
			continue
		}
		item.SupplierID = firstPositiveInt64(credential.SupplierID, item.SupplierID)
		item.RegistrationStatus = credential.Status
		item.RegistrationTaskID = credential.ExtensionTaskID
		item.RegistrationEmail = credential.Email
		item.RegistrationErrorCode = credential.ErrorCode
		item.RegistrationErrorMessage = credential.ErrorMessage
	}
	if isRegisteredDiscovery(item, nil) {
		item.RegistrationStatus = adminplusdomain.SupplierRegistrationStatusSucceeded
		item.RegistrationTaskID = 0
		item.RegistrationErrorCode = ""
		item.RegistrationErrorMessage = ""
	}
	return item
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

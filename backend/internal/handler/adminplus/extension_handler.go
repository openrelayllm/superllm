package adminplus

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	extensionapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/extension"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ExtensionHandler struct {
	service *extensionapp.Service
}

func NewExtensionHandler(service *extensionapp.Service) *ExtensionHandler {
	return &ExtensionHandler{service: service}
}

type createExtensionTaskRequest struct {
	SupplierID     int64          `json:"supplier_id" binding:"required"`
	Type           string         `json:"type" binding:"required"`
	Priority       int            `json:"priority"`
	MaxAttempts    int            `json:"max_attempts"`
	AvailableAfter string         `json:"available_after"`
	Payload        map[string]any `json:"payload"`
}

type claimExtensionTaskRequest struct {
	DeviceID        string   `json:"device_id" binding:"required"`
	Types           []string `json:"types"`
	LeaseTTLSeconds int64    `json:"lease_ttl_seconds"`
}

type captureSessionTaskRequest struct {
	SupplierID      int64          `json:"supplier_id" binding:"required"`
	DeviceID        string         `json:"device_id" binding:"required"`
	LeaseTTLSeconds int64          `json:"lease_ttl_seconds"`
	Payload         map[string]any `json:"payload"`
}

type extensionTaskLeaseRequest struct {
	DeviceID        string         `json:"device_id" binding:"required"`
	LeaseToken      string         `json:"lease_token" binding:"required"`
	LeaseTTLSeconds int64          `json:"lease_ttl_seconds"`
	Result          map[string]any `json:"result"`
	ErrorCode       string         `json:"error_code"`
	ErrorMessage    string         `json:"error_message"`
	RetryAfter      string         `json:"retry_after"`
}

func (h *ExtensionHandler) CreateTask(c *gin.Context) {
	var req createExtensionTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	availableAfter, ok := parseOptionalNamedTime(c, "available_after", req.AvailableAfter)
	if !ok {
		return
	}
	task, err := h.service.CreateTask(c.Request.Context(), extensionapp.CreateTaskInput{
		SupplierID:     req.SupplierID,
		Type:           adminplusdomain.ExtensionTaskType(req.Type),
		Priority:       req.Priority,
		MaxAttempts:    req.MaxAttempts,
		AvailableAfter: availableAfter,
		Payload:        req.Payload,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, task)
}

func (h *ExtensionHandler) ClaimTask(c *gin.Context) {
	var req claimExtensionTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	taskTypes := make([]adminplusdomain.ExtensionTaskType, 0, len(req.Types))
	for _, taskType := range req.Types {
		taskTypes = append(taskTypes, adminplusdomain.ExtensionTaskType(taskType))
	}
	task, err := h.service.ClaimTask(c.Request.Context(), extensionapp.ClaimTaskInput{
		DeviceID: req.DeviceID,
		Types:    taskTypes,
		LeaseTTL: secondsDuration(req.LeaseTTLSeconds),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, task)
}

func (h *ExtensionHandler) CreateCaptureSessionTask(c *gin.Context) {
	var req captureSessionTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	task, err := h.service.CreateLeasedTask(c.Request.Context(), extensionapp.CreateLeasedTaskInput{
		SupplierID: req.SupplierID,
		Type:       adminplusdomain.ExtensionTaskTypeCaptureSession,
		DeviceID:   req.DeviceID,
		LeaseTTL:   secondsDuration(req.LeaseTTLSeconds),
		Payload:    req.Payload,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, task)
}

func (h *ExtensionHandler) Heartbeat(c *gin.Context) {
	id, ok := parseExtensionTaskID(c)
	if !ok {
		return
	}
	var req extensionTaskLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	task, err := h.service.Heartbeat(c.Request.Context(), extensionapp.HeartbeatInput{
		TaskID:     id,
		DeviceID:   req.DeviceID,
		LeaseToken: req.LeaseToken,
		LeaseTTL:   secondsDuration(req.LeaseTTLSeconds),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, task)
}

func (h *ExtensionHandler) CompleteTask(c *gin.Context) {
	id, ok := parseExtensionTaskID(c)
	if !ok {
		return
	}
	var req extensionTaskLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	task, err := h.service.CompleteTask(c.Request.Context(), extensionapp.CompleteTaskInput{
		TaskID:     id,
		DeviceID:   req.DeviceID,
		LeaseToken: req.LeaseToken,
		Result:     req.Result,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, task)
}

func (h *ExtensionHandler) FailTask(c *gin.Context) {
	id, ok := parseExtensionTaskID(c)
	if !ok {
		return
	}
	var req extensionTaskLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	retryAfter, ok := parseOptionalNamedTime(c, "retry_after", req.RetryAfter)
	if !ok {
		return
	}
	task, err := h.service.FailTask(c.Request.Context(), extensionapp.FailTaskInput{
		TaskID:       id,
		DeviceID:     req.DeviceID,
		LeaseToken:   req.LeaseToken,
		ErrorCode:    req.ErrorCode,
		ErrorMessage: req.ErrorMessage,
		RetryAfter:   retryAfter,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, task)
}

func (h *ExtensionHandler) GetBrowserCredential(c *gin.Context) {
	id, ok := parseExtensionTaskID(c)
	if !ok {
		return
	}
	var req extensionTaskLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	credential, err := h.service.GetBrowserCredential(c.Request.Context(), extensionapp.BrowserCredentialInput{
		TaskID:     id,
		DeviceID:   req.DeviceID,
		LeaseToken: req.LeaseToken,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, credential)
}

func (h *ExtensionHandler) ListTasks(c *gin.Context) {
	page := parsePagination(c)
	items, err := h.service.ListTasks(c.Request.Context(), extensionapp.TaskFilter{
		SupplierID: parseInt64Query(c, "supplier_id"),
		Status:     adminplusdomain.ExtensionTaskStatus(c.Query("status")),
		Type:       adminplusdomain.ExtensionTaskType(c.Query("type")),
		Limit:      fetchLimitForPagination(page),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	paged, total := paginateSlice(items, page)
	response.Success(c, paginatedData(paged, total, page))
}

func (h *ExtensionHandler) Manifest(c *gin.Context) {
	info, err := loadExtensionManifest()
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, info)
}

func (h *ExtensionHandler) DownloadPackage(c *gin.Context) {
	adminPlusOrigin, ok := extensionPackageOrigin(c)
	if !ok {
		return
	}
	archive, info, err := buildExtensionZip(adminPlusOrigin)
	if response.ErrorFrom(c, err) {
		return
	}
	filename := "sub2api-plus-session-capture-" + info.Version + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/zip", archive)
}

func parseExtensionTaskID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid extension task id")
		return 0, false
	}
	return id, true
}

func secondsDuration(seconds int64) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

type extensionManifestInfo struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	Path        string   `json:"path"`
}

type extensionDefaultConfig struct {
	BaseURL string `json:"baseURL"`
}

const extensionDefaultConfigPath = "config/default-config.json"

func loadExtensionManifest() (*extensionManifestInfo, error) {
	root, err := extensionRoot()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return nil, extensionHandlerInternalError("failed to read extension manifest")
	}
	var raw struct {
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, extensionHandlerInternalError("failed to parse extension manifest")
	}
	return &extensionManifestInfo{
		Name:        raw.Name,
		Version:     raw.Version,
		Description: raw.Description,
		Permissions: raw.Permissions,
		Path:        root,
	}, nil
}

func buildExtensionZip(adminPlusOrigin string) ([]byte, *extensionManifestInfo, error) {
	info, err := loadExtensionManifest()
	if err != nil {
		return nil, nil, err
	}
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	written := map[string]struct{}{}
	err = filepath.WalkDir(info.Path, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			if entry.IsDir() && path != info.Path {
				return filepath.SkipDir
			}
			if !entry.IsDir() {
				return nil
			}
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(info.Path, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == extensionDefaultConfigPath && adminPlusOrigin != "" {
			return nil
		}
		writer, err := zipWriter.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		written[rel] = struct{}{}
		return err
	})
	if err == nil && adminPlusOrigin != "" {
		if _, exists := written[extensionDefaultConfigPath]; !exists {
			err = writeExtensionDefaultConfig(zipWriter, adminPlusOrigin)
		}
	}
	if closeErr := zipWriter.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, nil, extensionHandlerInternalError("failed to build extension package")
	}
	return buf.Bytes(), info, nil
}

func writeExtensionDefaultConfig(zipWriter *zip.Writer, adminPlusOrigin string) error {
	data, err := json.Marshal(extensionDefaultConfig{
		BaseURL: adminPlusOrigin,
	})
	if err != nil {
		return err
	}
	writer, err := zipWriter.Create(extensionDefaultConfigPath)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func extensionPackageOrigin(c *gin.Context) (string, bool) {
	if rawOrigin := strings.TrimSpace(c.Query("admin_plus_origin")); rawOrigin != "" {
		origin, ok := normalizeExtensionOrigin(rawOrigin)
		if !ok {
			response.BadRequest(c, "admin_plus_origin must be an http(s) origin")
			return "", false
		}
		return origin, true
	}
	if origin, ok := normalizeExtensionOrigin(originFromRequest(c.Request)); ok {
		return origin, true
	}
	return "", true
}

func originFromRequest(r *http.Request) string {
	host := firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	proto := firstHeaderValue(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	return proto + "://" + host
}

func firstHeaderValue(value string) string {
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}

func normalizeExtensionOrigin(rawOrigin string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawOrigin))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	return strings.TrimRight(parsed.Scheme+"://"+parsed.Host, "/"), true
}

func extensionRoot() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("ADMIN_PLUS_EXTENSION_DIR")); configured != "" {
		if stat, err := os.Stat(filepath.Join(configured, "manifest.json")); err == nil && !stat.IsDir() {
			return configured, nil
		}
		return "", extensionHandlerInternalError("ADMIN_PLUS_EXTENSION_DIR does not contain manifest.json")
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", extensionHandlerInternalError("failed to resolve working directory")
	}
	for {
		candidate := filepath.Join(wd, "..", "extension")
		if stat, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil && !stat.IsDir() {
			abs, _ := filepath.Abs(candidate)
			return abs, nil
		}
		candidate = filepath.Join(wd, "extension")
		if stat, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil && !stat.IsDir() {
			abs, _ := filepath.Abs(candidate)
			return abs, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}
	return "", extensionHandlerInternalError("extension directory was not found")
}

func extensionHandlerInternalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}

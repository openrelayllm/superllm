package adminplus

import (
	"strconv"
	"time"

	mailverificationapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/mailverification"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type MailVerificationHandler struct {
	service *mailverificationapp.Service
}

func NewMailVerificationHandler(service *mailverificationapp.Service) *MailVerificationHandler {
	return &MailVerificationHandler{service: service}
}

type mailCredentialRequest struct {
	Provider     adminplusdomain.MailVerificationProvider `json:"provider"`
	Name         string                                   `json:"name"`
	Email        string                                   `json:"email"`
	AccessToken  string                                   `json:"access_token"`
	RefreshToken string                                   `json:"refresh_token"`
	Scopes       []string                                 `json:"scopes"`
	Scope        string                                   `json:"scope"`
	TokenType    string                                   `json:"token_type"`
	ExpiresAt    *time.Time                               `json:"expires_at"`
	ExpiresIn    int                                      `json:"expires_in"`
	Metadata     map[string]string                        `json:"metadata"`
}

type mailOAuthAuthorizeRequest struct {
	Provider    adminplusdomain.MailVerificationProvider `json:"provider"`
	RedirectURI string                                   `json:"redirect_uri"`
	State       string                                   `json:"state"`
	LoginHint   string                                   `json:"login_hint"`
}

type mailOAuthExchangeRequest struct {
	Provider    adminplusdomain.MailVerificationProvider `json:"provider"`
	Code        string                                   `json:"code"`
	RedirectURI string                                   `json:"redirect_uri"`
	Name        string                                   `json:"name"`
}

type mailOAuthSettingsRequest struct {
	Provider            adminplusdomain.MailVerificationProvider `json:"provider"`
	ClientID            string                                   `json:"client_id"`
	ClientSecret        string                                   `json:"client_secret"`
	RedirectURI         string                                   `json:"redirect_uri"`
	FrontendRedirectURI string                                   `json:"frontend_redirect_uri"`
}

type mailVerificationCodeReadRequest struct {
	Provider            adminplusdomain.MailVerificationProvider `json:"provider"`
	CredentialID        int64                                    `json:"credential_id"`
	From                string                                   `json:"from"`
	Keywords            []string                                 `json:"keywords"`
	SupplierType        adminplusdomain.SupplierType             `json:"supplier_type"`
	ExpectedPurpose     string                                   `json:"expected_purpose"`
	SiteName            string                                   `json:"site_name"`
	TriggeredAt         *time.Time                               `json:"triggered_at"`
	TimeoutSeconds      int                                      `json:"timeout_seconds"`
	PollIntervalSeconds int                                      `json:"poll_interval_seconds"`
	MaxResults          int                                      `json:"max_results"`
}

type mailVerificationCodeSendTestRequest struct {
	CredentialID        int64                        `json:"credential_id"`
	SupplierType        adminplusdomain.SupplierType `json:"supplier_type"`
	ExpectedPurpose     string                       `json:"expected_purpose"`
	SiteName            string                       `json:"site_name"`
	TimeoutSeconds      int                          `json:"timeout_seconds"`
	PollIntervalSeconds int                          `json:"poll_interval_seconds"`
}

func (h *MailVerificationHandler) AuthorizeURL(c *gin.Context) {
	var req mailOAuthAuthorizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.AuthorizeURL(c.Request.Context(), mailverificationapp.OAuthAuthorizeInput{
		Provider:    req.Provider,
		RedirectURI: req.RedirectURI,
		State:       req.State,
		LoginHint:   req.LoginHint,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *MailVerificationHandler) ExchangeCode(c *gin.Context) {
	var req mailOAuthExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	credential, err := h.service.ExchangeCode(c.Request.Context(), mailverificationapp.ExchangeCodeInput{
		Provider:    req.Provider,
		Code:        req.Code,
		RedirectURI: req.RedirectURI,
		Name:        req.Name,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, credential)
}

func (h *MailVerificationHandler) OAuthSettings(c *gin.Context) {
	result, err := h.service.OAuthSettings(c.Request.Context(), adminplusdomain.MailVerificationProvider(c.Query("provider")))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *MailVerificationHandler) UpdateOAuthSettings(c *gin.Context) {
	var req mailOAuthSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.UpdateOAuthSettings(c.Request.Context(), mailverificationapp.UpdateOAuthSettingsInput{
		Provider:            req.Provider,
		ClientID:            req.ClientID,
		ClientSecret:        req.ClientSecret,
		RedirectURI:         req.RedirectURI,
		FrontendRedirectURI: req.FrontendRedirectURI,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *MailVerificationHandler) SaveCredential(c *gin.Context) {
	var req mailCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	credential, err := h.service.SaveCredential(c.Request.Context(), mailverificationapp.SaveCredentialInput{
		Provider:     req.Provider,
		Name:         req.Name,
		Email:        req.Email,
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		Scopes:       req.Scopes,
		Scope:        req.Scope,
		TokenType:    req.TokenType,
		ExpiresAt:    req.ExpiresAt,
		ExpiresIn:    req.ExpiresIn,
		Metadata:     req.Metadata,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, credential)
}

func (h *MailVerificationHandler) ListCredentials(c *gin.Context) {
	items, err := h.service.ListCredentials(c.Request.Context(), mailverificationapp.CredentialFilter{
		Provider: adminplusdomain.MailVerificationProvider(c.Query("provider")),
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, items)
}

func (h *MailVerificationHandler) CheckCredential(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid mail credential id")
		return
	}
	credential, err := h.service.CheckCredential(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, credential)
}

func (h *MailVerificationHandler) ReadVerificationCode(c *gin.Context) {
	var req mailVerificationCodeReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.ReadVerificationCode(c.Request.Context(), mailverificationapp.ReadVerificationCodeInput{
		Provider:            req.Provider,
		CredentialID:        req.CredentialID,
		From:                req.From,
		Keywords:            req.Keywords,
		SupplierType:        req.SupplierType,
		ExpectedPurpose:     req.ExpectedPurpose,
		SiteName:            req.SiteName,
		TriggeredAt:         req.TriggeredAt,
		TimeoutSeconds:      req.TimeoutSeconds,
		PollIntervalSeconds: req.PollIntervalSeconds,
		MaxResults:          req.MaxResults,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *MailVerificationHandler) SendTestVerificationCode(c *gin.Context) {
	var req mailVerificationCodeSendTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	result, err := h.service.SendAndReadTestVerificationCode(c.Request.Context(), mailverificationapp.SendTestVerificationCodeInput{
		CredentialID:        req.CredentialID,
		SupplierType:        req.SupplierType,
		ExpectedPurpose:     req.ExpectedPurpose,
		SiteName:            req.SiteName,
		TimeoutSeconds:      req.TimeoutSeconds,
		PollIntervalSeconds: req.PollIntervalSeconds,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

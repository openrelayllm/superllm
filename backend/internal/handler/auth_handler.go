package handler

import (
	"log/slog"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	cfg         *config.Config
	authService *service.AuthService
	settingSvc  *service.SettingService
	totpService *service.TotpService
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(cfg *config.Config, authService *service.AuthService, settingService *service.SettingService, totpService *service.TotpService) *AuthHandler {
	return &AuthHandler{
		cfg:         cfg,
		authService: authService,
		settingSvc:  settingService,
		totpService: totpService,
	}
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	TurnstileToken string `json:"turnstile_token"`
}

// AuthResponse 认证响应格式（匹配前端期望）
type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"` // 新增：Refresh Token
	ExpiresIn    int       `json:"expires_in,omitempty"`    // 新增：Access Token有效期（秒）
	TokenType    string    `json:"token_type"`
	User         *dto.User `json:"user"`
}

func ensureLoginUserActive(user *service.User) error {
	if user == nil {
		return infraerrors.Unauthorized("INVALID_USER", "user not found")
	}
	if !user.IsActive() {
		return service.ErrUserNotActive
	}
	return nil
}

// respondWithTokenPair 生成 Token 对并返回认证响应
// 如果 Token 对生成失败，回退到只返回 Access Token（向后兼容）
func (h *AuthHandler) respondWithTokenPair(c *gin.Context, user *service.User) {
	if err := ensureLoginUserActive(user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	tokenPair, err := h.authService.GenerateTokenPair(c.Request.Context(), user, "")
	if err != nil {
		slog.Error("failed to generate token pair", "error", err, "user_id", user.ID)
		// 回退到只返回Access Token
		token, tokenErr := h.authService.GenerateToken(user)
		if tokenErr != nil {
			response.InternalError(c, "Failed to generate token")
			return
		}
		response.Success(c, AuthResponse{
			AccessToken: token,
			TokenType:   "Bearer",
			User:        dto.UserFromService(user),
		})
		return
	}
	response.Success(c, AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		TokenType:    "Bearer",
		User:         dto.UserFromService(user),
	})
}

func ensureAdminUser(user *service.User) error {
	if user == nil {
		return infraerrors.Unauthorized("INVALID_USER", "user not found")
	}
	if user.IsAdmin() {
		return nil
	}
	return infraerrors.Forbidden("ADMIN_ONLY", "Only Sub2API administrators can access SuperLLM.")
}

// Login handles user login
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Turnstile 验证
	if err := h.authService.VerifyTurnstile(c.Request.Context(), req.TurnstileToken, ip.GetClientIP(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	token, user, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	_ = token // token 由 authService.Login 返回但此处由 respondWithTokenPair 重新生成

	if err := ensureAdminUser(user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Check if TOTP 2FA is enabled for this user
	if h.totpService != nil && user.TotpEnabled {
		// Create a temporary login session for 2FA
		tempToken, err := h.totpService.CreateLoginSession(c.Request.Context(), user.ID, user.Email)
		if err != nil {
			response.InternalError(c, "Failed to create 2FA session")
			return
		}

		response.Success(c, TotpLoginResponse{
			Requires2FA:     true,
			TempToken:       tempToken,
			UserEmailMasked: service.MaskEmail(user.Email),
		})
		return
	}

	h.respondWithTokenPair(c, user)
}

// TotpLoginResponse represents the response when 2FA is required
type TotpLoginResponse struct {
	Requires2FA     bool   `json:"requires_2fa"`
	TempToken       string `json:"temp_token,omitempty"`
	UserEmailMasked string `json:"user_email_masked,omitempty"`
}

// Login2FARequest represents the 2FA login request
type Login2FARequest struct {
	TempToken string `json:"temp_token" binding:"required"`
	TotpCode  string `json:"totp_code" binding:"required,len=6"`
}

// Login2FA completes the login with 2FA verification
// POST /api/v1/auth/login/2fa
func (h *AuthHandler) Login2FA(c *gin.Context) {
	var req Login2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	slog.Debug("login_2fa_request",
		"temp_token_len", len(req.TempToken),
		"totp_code_len", len(req.TotpCode))

	// Get the login session
	session, err := h.totpService.GetLoginSession(c.Request.Context(), req.TempToken)
	if err != nil || session == nil {
		tokenPrefix := ""
		if len(req.TempToken) >= 8 {
			tokenPrefix = req.TempToken[:8]
		}
		slog.Debug("login_2fa_session_invalid",
			"temp_token_prefix", tokenPrefix,
			"error", err)
		response.BadRequest(c, "Invalid or expired 2FA session")
		return
	}

	slog.Debug("login_2fa_session_found",
		"user_id", session.UserID,
		"email", session.Email)

	// Verify the TOTP code
	user, err := h.authService.GetIdentityByID(c.Request.Context(), session.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.totpService.VerifyIdentityCode(c.Request.Context(), user, req.TotpCode); err != nil {
		slog.Debug("login_2fa_verify_failed",
			"user_id", session.UserID,
			"error", err)
		response.ErrorFrom(c, err)
		return
	}

	// Get the user (before session deletion so we can check backend mode)
	if err := ensureLoginUserActive(user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if err := ensureAdminUser(user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Delete the login session (only after all checks pass)
	_ = h.totpService.DeleteLoginSession(c.Request.Context(), req.TempToken)

	h.respondWithTokenPair(c, user)
}

// GetCurrentUser handles getting current authenticated user
// GET /api/v1/auth/me
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	user, err := h.authService.GetIdentityByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	type UserResponse struct {
		dto.User
		RunMode string `json:"run_mode"`
	}

	runMode := config.RunModeStandard
	if h.cfg != nil {
		runMode = h.cfg.RunMode
	}

	response.Success(c, UserResponse{
		User:    *dto.UserFromService(user),
		RunMode: runMode,
	})
}

// ==================== Token Refresh Endpoints ====================

// RefreshTokenRequest 刷新Token请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenResponse 刷新Token响应
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // Access Token有效期（秒）
	TokenType    string `json:"token_type"`
}

// RefreshToken 刷新Token
// POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.authService.RefreshTokenPair(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if result.UserRole != service.RoleAdmin {
		response.Forbidden(c, "Only Sub2API administrators can access SuperLLM.")
		return
	}

	response.Success(c, RefreshTokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		TokenType:    "Bearer",
	})
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"` // 可选：撤销指定的Refresh Token
}

// LogoutResponse 登出响应
type LogoutResponse struct {
	Message string `json:"message"`
}

// Logout 用户登出
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	// 允许空请求体（向后兼容）
	_ = c.ShouldBindJSON(&req)

	// 如果提供了Refresh Token，撤销它
	if req.RefreshToken != "" {
		if err := h.authService.RevokeRefreshToken(c.Request.Context(), req.RefreshToken); err != nil {
			slog.Debug("failed to revoke refresh token", "error", err)
			// 不影响登出流程
		}
	}

	response.Success(c, LogoutResponse{
		Message: "Logged out successfully",
	})
}

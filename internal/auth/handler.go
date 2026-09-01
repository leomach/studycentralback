package auth

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/leomach/studycentralback/internal/platform"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterRateLimitedRoutes cadastra register/login — os alvos de força
// bruta de senha, por isso o chamador aplica rate limit só neste grupo.
func (h *Handler) RegisterRateLimitedRoutes(r *gin.RouterGroup) {
	r.POST("/auth/register", h.register)
	r.POST("/auth/login", h.login)
}

// RegisterPublicRoutes cadastra /refresh e /logout — não exigem access token
// (a credencial deles é o próprio refresh token no corpo) nem rate limit por
// IP: quem já está logado não deveria ser barrado por tentativas de login
// alheias vindas do mesmo IP (ex.: uma rede compartilhada).
func (h *Handler) RegisterPublicRoutes(r *gin.RouterGroup) {
	r.POST("/auth/refresh", h.refresh)
	r.POST("/auth/logout", h.logout)
}

// RegisterAdminRoutes espera um grupo já protegido por
// platform.RequireAdminSecret — não faz essa checagem aqui.
func (h *Handler) RegisterAdminRoutes(r *gin.RouterGroup) {
	r.POST("/users/:id/plan", h.promote)
}

// RegisterProtectedRoutes espera um grupo já protegido por
// platform.RequireAuth — não faz essa checagem aqui.
func (h *Handler) RegisterProtectedRoutes(r *gin.RouterGroup) {
	r.GET("/me", h.me)
}

func (h *Handler) me(c *gin.Context) {
	user, err := h.svc.Me(platform.UserID(c))
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) register(c *gin.Context) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	user, err := h.svc.Register(body.Name, body.Email, body.Password)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (h *Handler) login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	tokens, err := h.svc.Login(body.Email, body.Password)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	respondTokens(c, http.StatusOK, tokens)
}

func (h *Handler) refresh(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	tokens, err := h.svc.Refresh(body.RefreshToken)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	respondTokens(c, http.StatusOK, tokens)
}

func (h *Handler) logout(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	if err := h.svc.Logout(body.RefreshToken); err != nil {
		platform.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) promote(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		platform.Fail(c, platform.Invalid("id inválido"))
		return
	}

	if err := h.svc.PromoteToPremium(uint(id)); err != nil {
		platform.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func respondTokens(c *gin.Context, status int, tokens TokenPair) {
	c.JSON(status, gin.H{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
	})
}

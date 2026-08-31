package dashboard

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/leomach/studycentralback/internal/platform"
)

const defaultQueueMinutes = 40 // o bloco principal de estudo do dia

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/study/queue", h.queue)
	r.GET("/dashboard/overview", h.overview)
}

func (h *Handler) queue(c *gin.Context) {
	minutes, err := strconv.Atoi(c.Query("minutes"))
	if err != nil || minutes <= 0 {
		minutes = defaultQueueMinutes
	}

	items, err := h.svc.Queue(platform.UserID(c), minutes)
	if err != nil {
		platform.Fail(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"minutes": minutes, "items": items})
}

func (h *Handler) overview(c *gin.Context) {
	overview, err := h.svc.Overview(platform.UserID(c))
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, overview)
}

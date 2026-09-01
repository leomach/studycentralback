package question

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

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/questions", h.list)
	r.POST("/questions", h.create)
	r.GET("/questions/:id", h.find)
	r.PATCH("/questions/:id", h.update)
	r.DELETE("/questions/:id", h.delete)
	r.POST("/questions/:id/attempts", h.answer)
}

func (h *Handler) list(c *gin.Context) {
	f := ListFilter{
		SubjectID: uintQuery(c, "subject_id"),
		BancaID:   uintQuery(c, "banca_id"),
		ExamID:    uintQuery(c, "exam_id"),
		Format:    Format(c.Query("format")),
		Limit:     intQuery(c, "limit", 50),
	}

	questions, err := h.svc.List(platform.UserID(c), f)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, questions)
}

func (h *Handler) create(c *gin.Context) {
	var body struct {
		SubjectID     uint         `json:"subject_id"`
		BancaID       *uint        `json:"banca_id"`
		ExamID        *uint        `json:"exam_id"`
		Format        Format       `json:"format"`
		Statement     string       `json:"statement"`
		Alternatives  Alternatives `json:"alternatives"`
		CorrectAnswer string       `json:"correct_answer"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	q, err := h.svc.Create(platform.UserID(c), NewQuestion(body))
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, q)
}

func (h *Handler) answer(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	var body struct {
		Answer     string     `json:"answer"`
		Confidence Confidence `json:"confidence"`
		ClientID   string     `json:"client_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	attempt, err := h.svc.Answer(platform.UserID(c), id, body.ClientID, body.Answer, body.Confidence)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, attempt)
}

func (h *Handler) find(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	q, err := h.svc.FindByID(platform.UserID(c), id)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, q)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	var patch QuestionPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	q, err := h.svc.Update(platform.UserID(c), id, patch)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, q)
}

func (h *Handler) delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(platform.UserID(c), id); err != nil {
		platform.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func pathID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		platform.Fail(c, platform.Invalid("id inválido"))
		return 0, false
	}
	return uint(id), true
}

func uintQuery(c *gin.Context, key string) uint {
	v, err := strconv.ParseUint(c.Query(key), 10, 64)
	if err != nil {
		return 0
	}
	return uint(v)
}

func intQuery(c *gin.Context, key string, fallback int) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return fallback
	}
	return v
}

package flashcard

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

// defaultPageSize/maxPageSize governam a paginação de GET /flashcards, pelo
// mesmo motivo do par equivalente em question/handler.go.
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/flashcards", h.list)
	r.POST("/flashcards", h.create)
	// /flashcards/due antes de /flashcards/:id: o Gin casa a rota estática
	// primeiro, mas manter a ordem deixa a intenção explícita.
	r.GET("/flashcards/due", h.due)
	r.GET("/flashcards/:id", h.find)
	r.PATCH("/flashcards/:id", h.update)
	r.DELETE("/flashcards/:id", h.delete)
	r.POST("/flashcards/:id/reviews", h.grade)
}

func (h *Handler) list(c *gin.Context) {
	subjectID, _ := strconv.ParseUint(c.Query("subject_id"), 10, 64)

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil || limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	offset, err := strconv.Atoi(c.Query("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}

	cards, total, err := h.svc.List(platform.UserID(c), uint(subjectID), limit, offset)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":  cards,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handler) create(c *gin.Context) {
	var body struct {
		SubjectID        uint   `json:"subject_id"`
		Kind             Kind   `json:"kind"`
		Front            string `json:"front"`
		Back             string `json:"back"`
		SourceQuestionID *uint  `json:"source_question_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	card, err := h.svc.Create(platform.UserID(c), NewFlashcard(body))
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, card)
}

func (h *Handler) due(c *gin.Context) {
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 50
	}

	cards, err := h.svc.DueCards(platform.UserID(c), limit)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, cards)
}

func (h *Handler) grade(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	var body struct {
		Grade    Grade  `json:"grade"`
		ClientID string `json:"client_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	review, err := h.svc.Grade(platform.UserID(c), id, body.ClientID, body.Grade)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, review)
}

func (h *Handler) find(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	card, err := h.svc.FindByID(platform.UserID(c), id)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, card)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	var patch FlashcardPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	card, err := h.svc.Update(platform.UserID(c), id, patch)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, card)
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

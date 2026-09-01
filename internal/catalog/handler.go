package catalog

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

// RegisterRoutes deixa cada domínio dono das próprias rotas. Não há um
// urls.py central: o main só passa o grupo /api e cada handler se pendura.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/subjects", h.listSubjects)
	r.POST("/subjects", h.createSubject)
	r.PATCH("/subjects/:id", h.updateSubject)
	r.DELETE("/subjects/:id", h.deleteSubject)

	r.GET("/bancas", h.listBancas)
	r.POST("/bancas", h.createBanca)
	r.PATCH("/bancas/:id", h.updateBanca)
	r.DELETE("/bancas/:id", h.deleteBanca)

	r.GET("/exams", h.listExams)
	r.POST("/exams", h.createExam)
	r.PATCH("/exams/:id", h.updateExam)
	r.DELETE("/exams/:id", h.deleteExam)
}

func (h *Handler) listSubjects(c *gin.Context) {
	subjects, err := h.svc.ListSubjects()
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, subjects)
}

func (h *Handler) createSubject(c *gin.Context) {
	var body struct {
		Name     string `json:"name"`
		ParentID *uint  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	subject, err := h.svc.CreateSubject(body.Name, body.ParentID)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, subject)
}

func (h *Handler) updateSubject(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	var patch SubjectPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	subject, err := h.svc.UpdateSubject(id, patch)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, subject)
}

func (h *Handler) deleteSubject(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteSubject(id); err != nil {
		platform.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listBancas(c *gin.Context) {
	bancas, err := h.svc.ListBancas()
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, bancas)
}

func (h *Handler) createBanca(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	banca, err := h.svc.CreateBanca(body.Name)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, banca)
}

func (h *Handler) updateBanca(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	var body struct {
		Name *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	banca, err := h.svc.UpdateBanca(id, body.Name)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, banca)
}

func (h *Handler) deleteBanca(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteBanca(id); err != nil {
		platform.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listExams(c *gin.Context) {
	exams, err := h.svc.ListExams()
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, exams)
}

func (h *Handler) createExam(c *gin.Context) {
	var body struct {
		Name    string `json:"name"`
		BancaID uint   `json:"banca_id"`
		Year    int    `json:"year"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	exam, err := h.svc.CreateExam(body.Name, body.BancaID, body.Year)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, exam)
}

func (h *Handler) updateExam(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	var patch ExamPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		platform.Fail(c, platform.Invalid(err.Error()))
		return
	}

	exam, err := h.svc.UpdateExam(id, patch)
	if err != nil {
		platform.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, exam)
}

func (h *Handler) deleteExam(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteExam(id); err != nil {
		platform.Fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// pathID lê o :id da rota. Devolve ok=false já tendo respondido o erro — o
// handler que chamou só precisa dar return.
func pathID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		platform.Fail(c, platform.Invalid("id inválido"))
		return 0, false
	}
	return uint(id), true
}

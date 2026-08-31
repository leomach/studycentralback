package platform

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ErrorKind classifica a falha em termos que o PWA precisa distinguir para
// decidir o que fazer: corrigir o formulário, limpar o cache ou avisar que a
// operação esbarrou em dados existentes.
type ErrorKind string

const (
	KindInvalid  ErrorKind = "invalid"   // 400 — dado errado no pedido
	KindNotFound ErrorKind = "not_found" // 404 — não existe (ou não é seu)
	KindConflict ErrorKind = "conflict"  // 409 — esbarrou em dados em uso
)

// DomainError é o erro que um domínio devolve quando sabe o que aconteceu.
//
// Nota Go vs. Django: não existe ValidationError/Http404 lançados como exceção
// e capturados por um handler global. O erro é um valor comum que sobe pela
// pilha de retorno até o handler HTTP, que decide o status. Por isso ele
// carrega o Kind: é a informação que o handler usa para escolher o código.
type DomainError struct {
	Kind ErrorKind
	Msg  string
}

func (e *DomainError) Error() string { return e.Msg }

func Invalid(msg string) error  { return &DomainError{Kind: KindInvalid, Msg: msg} }
func NotFound(msg string) error { return &DomainError{Kind: KindNotFound, Msg: msg} }
func Conflict(msg string) error { return &DomainError{Kind: KindConflict, Msg: msg} }

var kindStatus = map[ErrorKind]int{
	KindInvalid:  http.StatusBadRequest,
	KindNotFound: http.StatusNotFound,
	KindConflict: http.StatusConflict,
}

// Fail traduz um erro em resposta HTTP. É o único lugar do sistema que decide
// status code, para que a API responda de forma previsível ao front.
func Fail(c *gin.Context, err error) {
	var domainErr *DomainError
	switch {
	case errors.As(err, &domainErr):
		respond(c, kindStatus[domainErr.Kind], domainErr.Kind, domainErr.Msg)

	// O Gorm devolve este erro em First() quando nada bate com o filtro.
	case errors.Is(err, gorm.ErrRecordNotFound):
		respond(c, http.StatusNotFound, KindNotFound, "recurso não encontrado")

	case isForeignKeyViolation(err):
		respond(c, http.StatusConflict, KindConflict,
			"registro está em uso por outros dados e não pode ser removido")

	default:
		// Erro não previsto: o detalhe fica no log do servidor, nunca na
		// resposta — mensagem de banco vazada não ajuda o front e expõe o
		// schema.
		log.Printf("erro não tratado em %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
		respond(c, http.StatusInternalServerError, "internal", "erro interno")
	}
}

func respond(c *gin.Context, status int, code ErrorKind, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg, "code": code})
}

// isForeignKeyViolation reconhece o 23503 do Postgres. errors.As desembrulha a
// cadeia de erros até achar um do tipo pedido — é como se inspeciona a causa
// original em Go, sem depender do texto da mensagem.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

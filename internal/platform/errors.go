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
	KindInvalid      ErrorKind = "invalid"      // 400 — dado errado no pedido
	KindUnauthorized ErrorKind = "unauthorized" // 401 — sem token válido
	KindForbidden    ErrorKind = "forbidden"    // 403 — autenticado, mas sem permissão
	KindNotFound     ErrorKind = "not_found"    // 404 — não existe (ou não é seu)
	KindConflict     ErrorKind = "conflict"     // 409 — esbarrou em dados em uso
	KindRateLimited  ErrorKind = "rate_limited" // 429 — muitas tentativas
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

func Invalid(msg string) error         { return &DomainError{Kind: KindInvalid, Msg: msg} }
func Unauthorized(msg string) error    { return &DomainError{Kind: KindUnauthorized, Msg: msg} }
func Forbidden(msg string) error       { return &DomainError{Kind: KindForbidden, Msg: msg} }
func NotFound(msg string) error        { return &DomainError{Kind: KindNotFound, Msg: msg} }
func Conflict(msg string) error        { return &DomainError{Kind: KindConflict, Msg: msg} }
func TooManyRequests(msg string) error { return &DomainError{Kind: KindRateLimited, Msg: msg} }

var kindStatus = map[ErrorKind]int{
	KindInvalid:      http.StatusBadRequest,
	KindUnauthorized: http.StatusUnauthorized,
	KindForbidden:    http.StatusForbidden,
	KindNotFound:     http.StatusNotFound,
	KindConflict:     http.StatusConflict,
	KindRateLimited:  http.StatusTooManyRequests,
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
	return pgErrorCode(err) == "23503"
}

// IsUniqueViolation reconhece o 23505 do Postgres (violação de UNIQUE).
// Exportada porque question e flashcard usam isso para detectar retentativa
// de escrita idempotente (mesmo client_id) sob concorrência.
func IsUniqueViolation(err error) bool {
	return pgErrorCode(err) == "23505"
}

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

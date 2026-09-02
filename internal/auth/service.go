package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/leomach/studycentralback/internal/platform"
)

const (
	minPasswordLen = 10
	// bcrypt trunca (não erra) qualquer coisa além de 72 bytes — melhor
	// recusar explicitamente do que aceitar uma senha cujos bytes extras são
	// ignorados em silêncio.
	maxPasswordBytes = 72
	bcryptCost       = 12

	refreshTokenTTL = 30 * 24 * time.Hour
)

// dummyHash existe só para Login gastar o mesmo tempo de CPU comparando senha
// mesmo quando o e-mail não existe — sem isto, a ausência de uma chamada a
// bcrypt.CompareHashAndPassword tornaria "conta não existe" mensurável pelo
// tempo de resposta, uma forma de enumerar e-mails cadastrados.
var dummyHash = mustHash("tempo-constante-nao-e-a-senha-de-ninguem")

func mustHash(s string) []byte {
	h, err := bcrypt.GenerateFromPassword([]byte(s), bcryptCost)
	if err != nil {
		panic(err) // só falha se bcryptCost for inválido — erro de programação, não de runtime.
	}
	return h
}

type Service struct {
	repo      *Repository
	jwtSecret string
	now       func() time.Time
}

func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{repo: repo, jwtSecret: jwtSecret, now: time.Now}
}

func (s *Service) Register(name, email, password string) (User, error) {
	name = strings.TrimSpace(name)
	email = normalizeEmail(email)

	if name == "" {
		return User{}, platform.Invalid("nome é obrigatório")
	}
	if !strings.Contains(email, "@") {
		return User{}, platform.Invalid("email inválido")
	}
	if len(password) < minPasswordLen {
		return User{}, platform.Invalid("senha precisa ter ao menos 10 caracteres")
	}
	if len(password) > maxPasswordBytes {
		return User{}, platform.Invalid("senha longa demais (máximo 72 caracteres)")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return User{}, err
	}

	user := User{Name: name, Email: email, PasswordHash: string(hash), Plan: PlanFree}
	if err := s.repo.CreateUser(&user); err != nil {
		if platform.IsUniqueViolation(err) {
			return User{}, platform.Conflict("já existe uma conta com este email")
		}
		return User{}, err
	}
	return user, nil
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// Login nunca diferencia "conta não existe" de "senha errada" — nem na
// mensagem, nem no tempo de resposta (por isso o bcrypt.CompareHashAndPassword
// contra dummyHash quando o email não é encontrado).
func (s *Service) Login(email, password string) (TokenPair, error) {
	email = normalizeEmail(email)
	const genericErr = "email ou senha inválidos"

	user, err := s.repo.FindUserByEmail(email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return TokenPair{}, platform.Unauthorized(genericErr)
	}
	if err != nil {
		return TokenPair{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return TokenPair{}, platform.Unauthorized(genericErr)
	}

	return s.issueTokenPair(user)
}

// Refresh troca um refresh token válido por um par novo (rotação a cada uso).
// Se o token apresentado já tiver sido usado antes (revoked_at preenchido),
// é sinal de que ele vazou e outra parte já o trocou — revoga a sessão
// inteira do usuário em vez de só recusar esta chamada.
func (s *Service) Refresh(rawToken string) (TokenPair, error) {
	hash := hashToken(rawToken)
	stored, err := s.repo.FindRefreshTokenByHash(hash)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return TokenPair{}, platform.Unauthorized("refresh token inválido")
	}
	if err != nil {
		return TokenPair{}, err
	}

	if stored.RevokedAt != nil {
		_ = s.repo.RevokeAllRefreshTokensForUser(stored.UserID)
		return TokenPair{}, platform.Unauthorized("sessão invalidada — faça login novamente")
	}
	if stored.ExpiresAt.Before(s.now()) {
		return TokenPair{}, platform.Unauthorized("refresh token expirado")
	}

	user, err := s.repo.FindUserByID(stored.UserID)
	if err != nil {
		return TokenPair{}, err
	}

	if err := s.repo.RevokeRefreshToken(stored.ID); err != nil {
		return TokenPair{}, err
	}
	return s.issueTokenPair(user)
}

func (s *Service) Logout(rawToken string) error {
	hash := hashToken(rawToken)
	stored, err := s.repo.FindRefreshTokenByHash(hash)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil // já não existe — logout é sempre "sucesso" do ponto de vista do cliente.
	}
	if err != nil {
		return err
	}
	return s.repo.RevokeRefreshToken(stored.ID)
}

func (s *Service) Me(userID uint) (User, error) {
	return s.repo.FindUserByID(userID)
}

func (s *Service) ListUsers() ([]User, error) {
	return s.repo.ListUsers()
}

// SetPlan é usado tanto pela rota de bootstrap protegida por ADMIN_SECRET
// (sempre chamando com PlanPremium) quanto pelo painel administrativo
// autenticado (aceita o plano vindo do corpo da requisição).
func (s *Service) SetPlan(userID uint, plan Plan) error {
	if !plan.Valid() {
		return platform.Invalid("plano inválido")
	}
	rows, err := s.repo.UpdatePlan(userID, plan)
	if err != nil {
		return err
	}
	if rows == 0 {
		return platform.NotFound("usuário não encontrado")
	}
	return nil
}

// GrantAdminByEmail é o bootstrap por e-mail (ver handler.bootstrapAdmin):
// não diferencia "email não existe" de outro erro na mensagem — não é alvo
// de força bruta como Login (a rota já exige ADMIN_SECRET antes de chegar
// aqui), então não há razão de segurança pra generalizar a mensagem, só
// clareza mesmo pra quem está rodando o bootstrap.
func (s *Service) GrantAdminByEmail(email string) error {
	email = normalizeEmail(email)
	user, err := s.repo.FindUserByEmail(email)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return platform.NotFound("nenhuma conta com este email")
	}
	if err != nil {
		return err
	}
	return s.SetAdmin(user.ID, true)
}

// SetAdmin concede ou revoga o papel de administrador de contas. Quem chama
// decide as travas de negócio (ex.: não deixar alguém remover o próprio
// papel) — aqui é só a escrita.
func (s *Service) SetAdmin(userID uint, isAdmin bool) error {
	rows, err := s.repo.UpdateAdmin(userID, isAdmin)
	if err != nil {
		return err
	}
	if rows == 0 {
		return platform.NotFound("usuário não encontrado")
	}
	return nil
}

func (s *Service) issueTokenPair(user User) (TokenPair, error) {
	access, err := platform.SignAccessToken(user.ID, string(user.Plan), user.IsAdmin, s.jwtSecret)
	if err != nil {
		return TokenPair{}, err
	}

	raw, hash, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	stored := RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: s.now().Add(refreshTokenTTL),
	}
	if err := s.repo.CreateRefreshToken(&stored); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{AccessToken: access, RefreshToken: raw}, nil
}

// newRefreshToken gera 32 bytes aleatórios (crypto/rand — não math/rand, que
// não é seguro para segredos). Devolve o valor em texto puro (vai pro
// cliente) e o hash SHA-256 dele (o único que é gravado no banco).
func newRefreshToken() (raw, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

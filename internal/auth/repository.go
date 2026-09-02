package auth

import (
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateUser(u *User) error { return r.db.Create(u).Error }

func (r *Repository) FindUserByEmail(email string) (User, error) {
	var u User
	err := r.db.Where("email = ?", email).First(&u).Error
	return u, err
}

func (r *Repository) FindUserByID(id uint) (User, error) {
	var u User
	err := r.db.First(&u, id).Error
	return u, err
}

func (r *Repository) UpdatePlan(userID uint, plan Plan) (int64, error) {
	res := r.db.Model(&User{}).Where("id = ?", userID).Update("plan", plan)
	return res.RowsAffected, res.Error
}

func (r *Repository) UpdateAdmin(userID uint, isAdmin bool) (int64, error) {
	res := r.db.Model(&User{}).Where("id = ?", userID).Update("is_admin", isAdmin)
	return res.RowsAffected, res.Error
}

// ListUsers devolve todas as contas, mais antiga primeiro — só o painel
// administrativo chama isto (RequireAdminRole), então nenhum filtro por
// user_id se aplica aqui, diferente do resto do sistema.
func (r *Repository) ListUsers() ([]User, error) {
	var users []User
	err := r.db.Order("id").Find(&users).Error
	return users, err
}

func (r *Repository) CreateRefreshToken(t *RefreshToken) error { return r.db.Create(t).Error }

func (r *Repository) FindRefreshTokenByHash(hash string) (RefreshToken, error) {
	var t RefreshToken
	err := r.db.Where("token_hash = ?", hash).First(&t).Error
	return t, err
}

func (r *Repository) RevokeRefreshToken(id uint) error {
	now := time.Now()
	return r.db.Model(&RefreshToken{}).Where("id = ?", id).Update("revoked_at", now).Error
}

// RevokeAllRefreshTokensForUser é chamado quando detectamos reuso de um
// refresh token já rotacionado — sinal de que o token vazou, então
// invalidamos a sessão inteira daquele usuário em todo dispositivo.
func (r *Repository) RevokeAllRefreshTokensForUser(userID uint) error {
	now := time.Now()
	return r.db.Model(&RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

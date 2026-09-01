package platform

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AccessTokenTTL é deliberadamente curto: um access token vazado (XSS, log
// mal feito) só serve por 15 minutos. Sessões longas vêm do refresh token,
// que é revogável — o access token não é.
const AccessTokenTTL = 15 * time.Minute

// Claims é o conteúdo assinado dentro do JWT. Plan viaja junto para que
// RequirePremium não precise consultar o banco a cada request — o preço é
// que promover alguém a premium só reflete depois que o access token atual
// expirar (no máximo AccessTokenTTL de atraso, aceitável dado o TTL curto).
type Claims struct {
	UserID uint   `json:"uid"`
	Plan   string `json:"plan"`
	jwt.RegisteredClaims
}

var ErrInvalidToken = errors.New("token inválido ou expirado")

func SignAccessToken(userID uint, plan, secret string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Plan:   plan,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func ParseAccessToken(tokenString, secret string) (Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		// Trava explícita no algoritmo esperado: sem isso, um token forjado
		// com alg=none ou outro algoritmo passaria a verificação.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

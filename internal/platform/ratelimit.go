package platform

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimit é uma janela deslizante em memória, por IP. Simples de propósito:
// o deploy alvo é um único processo (um VPS, sem múltiplas réplicas atrás de
// um load balancer — ver CLAUDE.md), então não há razão para um store
// distribuído (Redis) só para isto. Se um dia houver múltiplas réplicas, esta
// função precisa virar um store compartilhado — comentário para quem chegar
// aqui depois achando estranho o estado em memória de um servidor HTTP.
func RateLimit(max int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	hits := make(map[string][]time.Time)

	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now()

		mu.Lock()
		cutoff := now.Add(-window)
		recent := hits[key][:0]
		for _, t := range hits[key] {
			if t.After(cutoff) {
				recent = append(recent, t)
			}
		}
		blocked := len(recent) >= max
		if !blocked {
			recent = append(recent, now)
		}
		hits[key] = recent
		mu.Unlock()

		if blocked {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "muitas tentativas, tente novamente mais tarde",
				"code":  KindRateLimited,
			})
			return
		}
		c.Next()
	}
}

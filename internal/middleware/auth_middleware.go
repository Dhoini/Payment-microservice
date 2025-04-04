package middleware

import (
	"errors"
	"fmt" // Добавлен импорт
	"net/http"
	"strings"

	"github.com/Dhoini/Payment-microservice/internal/config"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/Dhoini/Payment-microservice/pkg/res"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ContextKey тип для ключей контекста во избежание коллизий.
type ContextKey string

const (
	// ContextUserIDKey ключ для хранения ID пользователя в контексте (используется HTTP middleware и gRPC interceptor).
	ContextUserIDKey ContextKey = "userID"
	authHeaderPrefix            = "Bearer "
)

type TokenValidator interface {
	Validate(tokenString string) (*TokenClaims, error)
}

type TokenClaims struct {
	UserEmail string `json:"email"`
	Scope     string `json:"scope"`
	jwt.RegisteredClaims
}

type JWTMiddleware struct {
	cfg       *config.Config
	log       *logger.Logger
	validator TokenValidator
}

func NewJWTMiddleware(cfg *config.Config, log *logger.Logger, validator TokenValidator) *JWTMiddleware {
	return &JWTMiddleware{
		cfg:       cfg,
		log:       log,
		validator: validator,
	}
}

func (m *JWTMiddleware) RequireAuth(requiredScopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			m.handleAuthError(c, "Missing authorization token")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, authHeaderPrefix)
		claims, err := m.validator.Validate(tokenString)
		if err != nil {
			m.handleAuthError(c, fmt.Sprintf("Token validation failed: %v", err)) // Добавляем ошибку в сообщение
			return
		}

		if len(requiredScopes) > 0 && !m.hasRequiredScope(claims.Scope, requiredScopes) {
			m.handleAuthError(c, "Insufficient token permissions")
			return
		}

		userID := claims.Subject
		if userID == "" {
			m.handleAuthError(c, "User ID (sub) missing in token")
			return
		}

		// Используем определенный ключ контекста
		c.Set(string(ContextUserIDKey), userID)
		c.Set("userEmail", claims.UserEmail) // Можно также добавить email в контекст, если нужно
		// Корректное логирование для вашего логгера
		m.log.Debugw("User authenticated via HTTP. UserID: %s", userID)
		c.Next()
	}
}

func (m *JWTMiddleware) hasRequiredScope(tokenScope string, requiredScopes []string) bool {
	if len(requiredScopes) == 0 {
		return true
	}
	for _, scope := range requiredScopes {
		if tokenScope == scope {
			return true
		}
	}
	return false
}

func (m *JWTMiddleware) handleAuthError(c *gin.Context, message string) {
	// Корректное логирование для вашего логгера
	m.log.Warnw("HTTP Authentication failed. Path: %s, Error: %s", c.Request.URL.Path, message)
	res.JsonResponse(c.Writer, res.ErrorResponse{
		Error:     message,
		ErrorCode: http.StatusUnauthorized,
	}, http.StatusUnauthorized)
	c.Abort()
}

// DefaultTokenValidator - реализация валидатора по умолчанию.
type DefaultTokenValidator struct {
	Secret []byte
}

func (v *DefaultTokenValidator) Validate(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.Secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, errors.New("malformed token")
		} else if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, errors.New("invalid token signature")
		} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, errors.New("token expired")
		} else {
			return nil, fmt.Errorf("invalid token: %w", err)
		}
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}

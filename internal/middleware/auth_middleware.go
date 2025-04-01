package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Dhoini/Payment-microservice/internal/config"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/Dhoini/Payment-microservice/pkg/res" // Ваш пакет для ответов

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// Ключ для сохранения userID в контексте Gin
	ContextUserIDKey = "userID"
	// Префикс заголовка Authorization
	authHeaderPrefix = "Bearer "
)

// AuthMiddleware содержит зависимости для middleware аутентификации.
type AuthMiddleware struct {
	cfg *config.Config
	log *logger.Logger
}

// NewAuthMiddleware создает экземпляр AuthMiddleware.
func NewAuthMiddleware(cfg *config.Config, log *logger.Logger) (*AuthMiddleware, error) {
	if cfg.Auth.JWTSecret == "" {
		log.Errorw("JWT secret key (auth.jwtSecret) is not configured")
		return nil, errors.New("JWT secret key is not configured")
	}
	return &AuthMiddleware{
		cfg: cfg,
		log: log,
	}, nil
}

// RequireAuth - это Gin middleware для проверки JWT аутентификации.
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		const handlerName = "[AuthMiddleware]" // Контекст для логов

		// 1. Получаем заголовок Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			m.log.Warnw(handlerName + " Authorization header missing")
			// Используем pkg/res для ответа
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Authorization header required"}, http.StatusUnauthorized)
			c.Abort() // Прерываем цепочку обработки
			return
		}

		// 2. Проверяем префикс Bearer и извлекаем токен
		if !strings.HasPrefix(authHeader, authHeaderPrefix) {
			m.log.Warnw(handlerName + " Invalid Authorization header format (Bearer prefix missing)")
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Invalid token format"}, http.StatusUnauthorized)
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(authHeader, authHeaderPrefix)

		// 3. Парсим и валидируем JWT токен
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Проверяем метод подписи (например, HMAC)
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Возвращаем секретный ключ для проверки подписи
			return []byte(m.cfg.Auth.JWTSecret), nil
		})

		// Обработка ошибок валидации токена
		if err != nil {
			logFields := []interface{}{"error", err}
			errMsg := "Invalid token"
			if errors.Is(err, jwt.ErrTokenMalformed) {
				errMsg = "Malformed token"
				m.log.Warnw(handlerName+" Malformed token", logFields...)
			} else if errors.Is(err, jwt.ErrTokenExpired) {
				errMsg = "Token expired"
				m.log.Warnw(handlerName+" Token expired", logFields...)
			} else if errors.Is(err, jwt.ErrTokenNotValidYet) {
				errMsg = "Token not active yet"
				m.log.Warnw(handlerName+" Token not active yet", logFields...)
			} else {
				// Другие ошибки (например, ошибка проверки подписи)
				m.log.Errorw(handlerName+" Token validation error", logFields...)
			}
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: errMsg}, http.StatusUnauthorized)
			c.Abort()
			return
		}

		// 4. Проверяем валидность токена и извлекаем claims (полезную нагрузку)
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			// 5. Извлекаем UserID из claims (предполагаем, что он там есть как "sub" или "user_id")
			userIDValue, exists := claims["user_id"] // Или "sub", зависит от того, как генерируется токен
			if !exists {
				m.log.Errorw(handlerName + " user_id claim missing in token")
				res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Invalid token claims"}, http.StatusUnauthorized)
				c.Abort()
				return
			}

			userID, ok := userIDValue.(string)
			if !ok {
				m.log.Errorw(handlerName + " user_id claim is not a string in token")
				res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Invalid token claims format"}, http.StatusUnauthorized)
				c.Abort()
				return
			}

			// 6. Сохраняем UserID в контексте Gin для следующих обработчиков
			c.Set(ContextUserIDKey, userID)
			m.log.Debugw(handlerName+" User authenticated successfully", "userID", userID)

			// Переходим к следующему обработчику
			c.Next()
		} else {
			m.log.Warnw(handlerName + " Invalid token (claims invalid or token not valid)")
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Invalid token"}, http.StatusUnauthorized)
			c.Abort()
		}
	}
}

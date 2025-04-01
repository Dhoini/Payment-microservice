package middleware

import (
	"time"

	"github.com/Dhoini/Payment-microservice/pkg/logger" // Ваш логгер
	"github.com/gin-gonic/gin"
)

// RequestLogger - Gin middleware для логирования запросов с использованием вашего логгера.
func RequestLogger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Время начала обработки запроса
		start := time.Now()

		// Путь запроса
		path := c.Request.URL.Path
		// Сырой query string, если есть
		rawQuery := c.Request.URL.RawQuery
		if rawQuery != "" {
			path = path + "?" + rawQuery
		}

		// Обрабатываем запрос следующим middleware/обработчиком
		c.Next()

		// Время окончания обработки
		end := time.Now()
		latency := end.Sub(start)

		// Получаем детали ответа
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		userAgent := c.Request.UserAgent()
		// Размер ответа (если Content-Length известен)
		// responseSize := c.Writer.Size() // Не всегда точно или доступно

		// Получаем ошибки, если они были добавлены в контекст Gin обработчиками
		// err := c.Errors.ByType(gin.ErrorTypePrivate).String() // Пример получения ошибок

		// Логируем всю информацию
		log.Infow("Request handled",
			"status_code", statusCode,
			"method", method,
			"path", path,
			"latency_ms", latency.Milliseconds(), // Время обработки в миллисекундах
			"client_ip", clientIP,
			"user_agent", userAgent,
			// "response_size", responseSize,
			// "errors", err, // Логируем ошибки, если они есть
		)
	}
}

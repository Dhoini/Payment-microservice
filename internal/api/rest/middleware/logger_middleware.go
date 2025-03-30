package middleware

import (
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
	"time"
)

// LoggerMiddleware создает middleware для логирования запросов
func LoggerMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Время начала запроса
		startTime := time.Now()

		// Обработка запроса
		c.Next()

		// Время завершения запроса
		endTime := time.Now()
		// Длительность запроса
		latencyTime := endTime.Sub(startTime)

		// Получаем код статуса
		statusCode := c.Writer.Status()

		// Логируем информацию о запросе
		switch {
		case statusCode >= 500:
			log.Error("[%s] %s %d %s %s",
				c.Request.Method,
				c.Request.RequestURI,
				statusCode,
				latencyTime.String(),
				c.ClientIP(),
			)
		case statusCode >= 400:
			log.Warn("[%s] %s %d %s %s",
				c.Request.Method,
				c.Request.RequestURI,
				statusCode,
				latencyTime.String(),
				c.ClientIP(),
			)
		default:
			log.Info("[%s] %s %d %s %s",
				c.Request.Method,
				c.Request.RequestURI,
				statusCode,
				latencyTime.String(),
				c.ClientIP(),
			)
		}
	}
}

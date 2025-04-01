package res

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// ErrorResponse представляет формат JSON-ответа для ошибок.
type ErrorResponse struct {
	Error     string `json:"error"`                // Сообщение об ошибке (для пользователя)
	ErrorCode int    `json:"error_code,omitempty"` // Код ошибки (для программной обработки)
	Details   any    `json:"details,omitempty"`    // Детали ошибки (например, ошибки валидации)
	DebugInfo string `json:"debug_info,omitempty"` // Отладочная информация (ТОЛЬКО в development среде!)
}

// JsonResponse отправляет JSON-ответ с заданным статусом.
func JsonResponse(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// JsonErrorResponse отправляет JSON ответ ошибки.
func JsonErrorResponse(w http.ResponseWriter, errResponse ErrorResponse, status int, log *zap.Logger) {
	JsonResponse(w, errResponse, status)
	log.Error("Ошибка ответа", zap.Any("ошибка", errResponse))
}

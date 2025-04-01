package req

import (
	"encoding/json"
	"github.com/Dhoini/Payment-microservice/pkg/res"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
	"io"
	"net/http"
)

// Decode декодирует JSON из io.ReadCloser в структуру типа T.
func Decode[T any](body io.ReadCloser) (T, error) {
	var payload T
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return payload, err
	}
	return payload, nil
}

// IsValid валидирует структуру типа T.
func IsValid[T any](payload T) error {
	validate := validator.New()
	err := validate.Struct(payload)
	return err
}

// HandleBody декодирует, валидирует и обрабатывает тело запроса.
func HandleBody[T any](w *http.ResponseWriter, r *http.Request, log *zap.Logger) (*T, error) {
	body, err := Decode[T](r.Body)
	if err != nil {
		log.Error("Ошибка декодирования тела запроса", zap.Error(err)) // Добавляем логирование
		res.JsonResponse(*w, res.ErrorResponse{Error: "Некорректный формат запроса"}, http.StatusUnprocessableEntity)
		return nil, err
	}

	err = IsValid(body)
	if err != nil {
		log.Error("Ошибка валидации тела запроса", zap.Error(err)) // Добавляем логирование
		res.JsonResponse(*w, res.ErrorResponse{Error: "Некорректные данные запроса"}, http.StatusUnprocessableEntity)
		return nil, err
	}
	return &body, nil
}

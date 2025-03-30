package domain

import (
	"errors"
	"fmt"
)

// Application errors
var (
	// ErrNotFound запись не найдена
	ErrNotFound = errors.New("record not found")

	// ErrDuplicate дубликат записи
	ErrDuplicate = errors.New("duplicate record")

	// ErrInvalidInput неверные входные данные
	ErrInvalidInput = errors.New("invalid input data")

	// ErrUnauthenticated пользователь не аутентифицирован
	ErrUnauthenticated = errors.New("unauthenticated")

	// ErrUnauthorized пользователь не авторизован
	ErrUnauthorized = errors.New("unauthorized")

	// ErrInternal внутренняя ошибка
	ErrInternal = errors.New("internal error")

	// ErrInvalidOperation неверная операция
	ErrInvalidOperation = errors.New("invalid operation")

	// ErrTimeoutExceeded превышено время ожидания
	ErrTimeoutExceeded = errors.New("timeout exceeded")

	// ErrExternalServiceUnavailable внешний сервис недоступен
	ErrExternalServiceUnavailable = errors.New("external service unavailable")

	// ErrInsufficientFunds недостаточно средств
	ErrInsufficientFunds = errors.New("insufficient funds")

	// ErrPaymentFailed платеж не прошел
	ErrPaymentFailed = errors.New("payment failed")

	// ErrPaymentCancelled платеж отменен
	ErrPaymentCancelled = errors.New("payment cancelled")

	// ErrSubscriptionCancelled подписка отменена
	ErrSubscriptionCancelled = errors.New("subscription cancelled")

	// ErrCustomerNotFound клиент не найден
	ErrCustomerNotFound = errors.New("customer not found")

	// ErrPaymentMethodNotFound метод оплаты не найден
	ErrPaymentMethodNotFound = errors.New("payment method not found")

	// ErrSubscriptionPlanNotFound план подписки не найден
	ErrSubscriptionPlanNotFound = errors.New("subscription plan not found")

	// ErrWebhookValidationFailed не удалось проверить подпись вебхука
	ErrWebhookValidationFailed = errors.New("webhook validation failed")

	// ErrUnsupportedPaymentMethod неподдерживаемый метод оплаты
	ErrUnsupportedPaymentMethod = errors.New("unsupported payment method")

	// ErrUnsupportedCurrency неподдерживаемая валюта
	ErrUnsupportedCurrency = errors.New("unsupported currency")
)

// PaymentError представляет ошибку платежа
type PaymentError struct {
	Code        string
	Message     string
	PaymentID   string
	StatusCode  int
	OriginalErr error
}

// Error реализует интерфейс error
func (e *PaymentError) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("payment error [%s]: %s: %v (payment_id: %s)", e.Code, e.Message, e.OriginalErr, e.PaymentID)
	}
	return fmt.Sprintf("payment error [%s]: %s (payment_id: %s)", e.Code, e.Message, e.PaymentID)
}

// Unwrap возвращает оригинальную ошибку
func (e *PaymentError) Unwrap() error {
	return e.OriginalErr
}

// NewPaymentError создает новую ошибку платежа
func NewPaymentError(code, message, paymentID string, statusCode int, err error) *PaymentError {
	return &PaymentError{
		Code:        code,
		Message:     message,
		PaymentID:   paymentID,
		StatusCode:  statusCode,
		OriginalErr: err,
	}
}

// SubscriptionError представляет ошибку подписки
type SubscriptionError struct {
	Code           string
	Message        string
	SubscriptionID string
	StatusCode     int
	OriginalErr    error
}

// Error реализует интерфейс error
func (e *SubscriptionError) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("subscription error [%s]: %s: %v (subscription_id: %s)", e.Code, e.Message, e.OriginalErr, e.SubscriptionID)
	}
	return fmt.Sprintf("subscription error [%s]: %s (subscription_id: %s)", e.Code, e.Message, e.SubscriptionID)
}

// Unwrap возвращает оригинальную ошибку
func (e *SubscriptionError) Unwrap() error {
	return e.OriginalErr
}

// NewSubscriptionError создает новую ошибку подписки
func NewSubscriptionError(code, message, subscriptionID string, statusCode int, err error) *SubscriptionError {
	return &SubscriptionError{
		Code:           code,
		Message:        message,
		SubscriptionID: subscriptionID,
		StatusCode:     statusCode,
		OriginalErr:    err,
	}
}

// ValidationError представляет ошибку валидации
type ValidationError struct {
	Field   string
	Message string
}

// ValidationErrors представляет набор ошибок валидации
type ValidationErrors []ValidationError

// Error реализует интерфейс error
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "validation failed"
	}

	if len(e) == 1 {
		return fmt.Sprintf("validation failed: %s - %s", e[0].Field, e[0].Message)
	}

	return fmt.Sprintf("validation failed: %d errors", len(e))
}

// Add добавляет ошибку валидации
func (e *ValidationErrors) Add(field, message string) {
	*e = append(*e, ValidationError{Field: field, Message: message})
}

// HasErrors проверяет наличие ошибок
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Fields возвращает список полей с ошибками
func (e ValidationErrors) Fields() []string {
	fields := make([]string, len(e))
	for i, err := range e {
		fields[i] = err.Field
	}
	return fields
}

// GetByField возвращает сообщение об ошибке для указанного поля
func (e ValidationErrors) GetByField(field string) string {
	for _, err := range e {
		if err.Field == field {
			return err.Message
		}
	}
	return ""
}

// ExternalServiceError представляет ошибку внешнего сервиса
type ExternalServiceError struct {
	Service     string
	Code        string
	Message     string
	StatusCode  int
	OriginalErr error
}

// Error реализует интерфейс error
func (e *ExternalServiceError) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("%s service error [%s]: %s: %v", e.Service, e.Code, e.Message, e.OriginalErr)
	}
	return fmt.Sprintf("%s service error [%s]: %s", e.Service, e.Code, e.Message)
}

// Unwrap возвращает оригинальную ошибку
func (e *ExternalServiceError) Unwrap() error {
	return e.OriginalErr
}

// NewExternalServiceError создает новую ошибку внешнего сервиса
func NewExternalServiceError(service, code, message string, statusCode int, err error) *ExternalServiceError {
	return &ExternalServiceError{
		Service:     service,
		Code:        code,
		Message:     message,
		StatusCode:  statusCode,
		OriginalErr: err,
	}
}

// NotFoundError представляет ошибку "не найдено"
type NotFoundError struct {
	Entity string
	ID     string
}

// Error реализует интерфейс error
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with ID %s not found", e.Entity, e.ID)
}

// Is проверяет, является ли ошибка ошибкой типа "не найдено"
func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// NewNotFoundError создает новую ошибку "не найдено"
func NewNotFoundError(entity, id string) *NotFoundError {
	return &NotFoundError{
		Entity: entity,
		ID:     id,
	}
}

// DuplicateError представляет ошибку дубликата
type DuplicateError struct {
	Entity string
	Field  string
	Value  string
}

// Error реализует интерфейс error
func (e *DuplicateError) Error() string {
	return fmt.Sprintf("%s with %s '%s' already exists", e.Entity, e.Field, e.Value)
}

// Is проверяет, является ли ошибка ошибкой дубликата
func (e *DuplicateError) Is(target error) bool {
	return target == ErrDuplicate
}

// NewDuplicateError создает новую ошибку дубликата
func NewDuplicateError(entity, field, value string) *DuplicateError {
	return &DuplicateError{
		Entity: entity,
		Field:  field,
		Value:  value,
	}
}

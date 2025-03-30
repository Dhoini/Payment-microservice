package repository

import "errors"

var (
	// ErrNotFound запись не найдена
	ErrNotFound = errors.New("record not found")

	// ErrDuplicate дубликат записи
	ErrDuplicate = errors.New("duplicate record")

	// ErrInvalidData неверные данные
	ErrInvalidData = errors.New("invalid data")
)

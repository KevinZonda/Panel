package apperr

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserDisabled       = errors.New("user is disabled")
	ErrValidation         = errors.New("validation failed")
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource conflict")
	ErrForbidden          = errors.New("forbidden")
	ErrDependency         = errors.New("dependency error")
)

func Validationf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

func NotFoundf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrNotFound, fmt.Sprintf(format, args...))
}

func Conflictf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, args...))
}

func Forbiddenf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrForbidden, fmt.Sprintf(format, args...))
}

func Dependencyf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrDependency, fmt.Sprintf(format, args...))
}

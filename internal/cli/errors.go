package cli

import (
	"errors"
	"fmt"
)

const (
	ExitSuccess    = 0
	ExitParameter  = 2
	ExitDependency = 3
	ExitRuntime    = 4
)

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func NewParameterError(format string, args ...any) error {
	return &ExitError{
		Code: ExitParameter,
		Err:  fmt.Errorf(format, args...),
	}
}

func NewDependencyError(format string, args ...any) error {
	return &ExitError{
		Code: ExitDependency,
		Err:  fmt.Errorf(format, args...),
	}
}

func NewRuntimeError(format string, args ...any) error {
	return &ExitError{
		Code: ExitRuntime,
		Err:  fmt.Errorf(format, args...),
	}
}

func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	return ExitRuntime
}

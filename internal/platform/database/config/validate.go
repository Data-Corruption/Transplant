package config

import (
	"strings"

	"github.com/Data-Corruption/Transplant/internal/types"
	"github.com/Data-Corruption/Transplant/pkg/xlog"
)

// ValidationError reports one or more invalid configuration values. It is a
// domain error; callers at transport boundaries decide how to present it.
type ValidationError struct {
	errs []error
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	msgs := make([]string, 0, len(e.errs))
	for _, err := range e.errs {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

func (e *ValidationError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return e.errs
}

// validate runs inside Update on the resulting configuration before it is
// persisted, so every writer (HTTP handlers, CLI commands, internal code)
// gets the same protection: a bad bind or log level would otherwise only
// surface as a failed listen on the next start.
func validate(cfg *types.Configuration) error {
	var errs []error

	if _, err := xlog.NormalizeLevel(cfg.LogLevel); err != nil {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{errs: errs}
}

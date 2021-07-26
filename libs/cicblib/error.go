package cicblib

import "errors"

var (
	ErrMissingField   = errors.New("Missing field")
	ErrEmptyContent   = errors.New("Empty content")
	ErrNoCustomer     = errors.New("no such customer")
	ErrInvalidSession = errors.New("Invalid session")
	ErrTimeout        = errors.New("Timeout")
)

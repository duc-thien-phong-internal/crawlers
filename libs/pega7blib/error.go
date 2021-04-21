package pega7lib

import "errors"

var (
	ErrMissingField   = errors.New("Missing field")
	ErrEmptyContent   = errors.New("Empty content")
	ErrInvalidSession = errors.New("Invalid session")
	ErrTimeout        = errors.New("Timeout")
	ErrNoProducer     = errors.New("no producer")
)

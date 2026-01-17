package repository

import "errors"

// ErrNotFound is returned when a requested entity is not found.
var ErrNotFound = errors.New("entity not found")

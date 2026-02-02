package repository

import "errors"

// ErrNotFound is returned when a requested entity is not found.
var ErrNotFound = errors.New("entity not found")

// ErrAlreadyExists is returned when a unique constraint is violated.
var ErrAlreadyExists = errors.New("entity already exists")

package domain

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrAlreadyUsed  = errors.New("already used")
	ErrExpired      = errors.New("expired")
	ErrInvalidState = errors.New("invalid state")
)

package domain

import "errors"

var (
	ErrBlobNotFound     = errors.New("blob not found")
	ErrAlreadyCommitted = errors.New("blob already committed")
	ErrInvalidBlobID    = errors.New("invalid blob_id: must be 64-char lowercase hex")
	ErrBlobPending      = errors.New("blob is in PENDING state")
)

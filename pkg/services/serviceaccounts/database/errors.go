package database

import (
	"fmt"

	"github.com/grafana/grafana/pkg/models"
)

type ErrSAInvalidName struct {
}

func (e *ErrSAInvalidName) Error() string {
	return "service account name already in use"
}

func (e *ErrSAInvalidName) Unwrap() error {
	return models.ErrUserAlreadyExists
}

type ErrMissingSAToken struct {
}

func (e *ErrMissingSAToken) Error() string {
	return "service account token not found"
}

func (e *ErrMissingSAToken) Unwrap() error {
	return models.ErrApiKeyNotFound
}

type ErrInvalidExpirationSAToken struct {
}

func (e *ErrInvalidExpirationSAToken) Error() string {
	return "service account token not found"
}

func (e *ErrInvalidExpirationSAToken) Unwrap() error {
	return models.ErrInvalidApiKeyExpiration
}

type ErrDuplicateSAToken struct {
	name string
}

func (e *ErrDuplicateSAToken) Error() string {
	return fmt.Sprintf("service account token %s already exists in the organization", e.name)
}

func (e *ErrDuplicateSAToken) Unwrap() error {
	return models.ErrDuplicateApiKey
}

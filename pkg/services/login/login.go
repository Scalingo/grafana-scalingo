package login

import (
	"context"
	"errors"

	"github.com/grafana/grafana/pkg/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUsersQuotaReached  = errors.New("users quota reached")
	ErrGettingUserQuota   = errors.New("error getting user quota")
	ErrSignupNotAllowed   = errors.New("system administrator has disabled signup")
)

type TeamSyncFunc func(user *models.User, externalUser *models.ExternalUserInfo) error

type Service interface {
	CreateUser(cmd models.CreateUserCommand) (*models.User, error)
	UpsertUser(ctx context.Context, cmd *models.UpsertUserCommand) error
	DisableExternalUser(ctx context.Context, username string) error
	SetTeamSyncFunc(TeamSyncFunc)
}

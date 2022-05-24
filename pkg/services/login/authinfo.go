package login

import (
	"context"

	"github.com/grafana/grafana/pkg/models"
)

type AuthInfoService interface {
	LookupAndUpdate(ctx context.Context, query *models.GetUserByAuthInfoQuery) (*models.User, error)
	GetAuthInfo(ctx context.Context, query *models.GetAuthInfoQuery) error
	GetExternalUserInfoByLogin(ctx context.Context, query *models.GetExternalUserInfoByLoginQuery) error
	SetAuthInfo(ctx context.Context, cmd *models.SetAuthInfoCommand) error
	UpdateAuthInfo(ctx context.Context, cmd *models.UpdateAuthInfoCommand) error
}

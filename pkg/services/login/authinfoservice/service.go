package authinfoservice

import (
	"context"
	"errors"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/usagestats"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/login"
	"github.com/grafana/grafana/pkg/services/user"
)

const genericOAuthModule = "oauth_generic_oauth"

type Implementation struct {
	UserProtectionService login.UserProtectionService
	authInfoStore         login.Store
	logger                log.Logger
}

func ProvideAuthInfoService(userProtectionService login.UserProtectionService, authInfoStore login.Store, usageStats usagestats.Service) *Implementation {
	s := &Implementation{
		UserProtectionService: userProtectionService,
		authInfoStore:         authInfoStore,
		logger:                log.New("login.authinfo"),
	}
	usageStats.RegisterMetricsFunc(authInfoStore.CollectLoginStats)
	return s
}

func (s *Implementation) LookupAndFix(ctx context.Context, query *models.GetUserByAuthInfoQuery) (bool, *user.User, *models.UserAuth, error) {
	authQuery := &models.GetAuthInfoQuery{}

	// Try to find the user by auth module and id first
	if query.AuthModule != "" && query.AuthId != "" {
		authQuery.AuthModule = query.AuthModule
		authQuery.AuthId = query.AuthId

		err := s.authInfoStore.GetAuthInfo(ctx, authQuery)
		if !errors.Is(err, user.ErrUserNotFound) {
			if err != nil {
				return false, nil, nil, err
			}

			// if user id was specified and doesn't match the user_auth entry, remove it
			if query.UserLookupParams.UserID != nil &&
				*query.UserLookupParams.UserID != 0 &&
				*query.UserLookupParams.UserID != authQuery.Result.UserId {
				if err := s.authInfoStore.DeleteAuthInfo(ctx, &models.DeleteAuthInfoCommand{
					UserAuth: authQuery.Result,
				}); err != nil {
					s.logger.Error("Error removing user_auth entry", "error", err)
				}

				return false, nil, nil, user.ErrUserNotFound
			} else {
				usr, err := s.authInfoStore.GetUserById(ctx, authQuery.Result.UserId)
				if err != nil {
					if errors.Is(err, user.ErrUserNotFound) {
						// if the user has been deleted then remove the entry
						if errDel := s.authInfoStore.DeleteAuthInfo(ctx, &models.DeleteAuthInfoCommand{
							UserAuth: authQuery.Result,
						}); errDel != nil {
							s.logger.Error("Error removing user_auth entry", "error", errDel)
						}

						return false, nil, nil, user.ErrUserNotFound
					}

					return false, nil, nil, err
				}

				return true, usr, authQuery.Result, nil
			}
		}
	}

	return false, nil, nil, user.ErrUserNotFound
}

func (s *Implementation) LookupByOneOf(ctx context.Context, params *models.UserLookupParams) (*user.User, error) {
	var usr *user.User
	var err error

	// If not found, try to find the user by id
	if params.UserID != nil && *params.UserID != 0 {
		usr, err = s.authInfoStore.GetUserById(ctx, *params.UserID)
		if err != nil && !errors.Is(err, user.ErrUserNotFound) {
			return nil, err
		}
	}

	// If not found, try to find the user by email address
	if usr == nil && params.Email != nil && *params.Email != "" {
		usr, err = s.authInfoStore.GetUserByEmail(ctx, *params.Email)
		if err != nil && !errors.Is(err, user.ErrUserNotFound) {
			return nil, err
		}
	}

	// If not found, try to find the user by login
	if usr == nil && params.Login != nil && *params.Login != "" {
		usr, err = s.authInfoStore.GetUserByLogin(ctx, *params.Login)
		if err != nil && !errors.Is(err, user.ErrUserNotFound) {
			return nil, err
		}
	}

	if usr == nil {
		return nil, user.ErrUserNotFound
	}

	return usr, nil
}

func (s *Implementation) GenericOAuthLookup(ctx context.Context, authModule string, authId string, userID int64) (*models.UserAuth, error) {
	if authModule == genericOAuthModule && userID != 0 {
		authQuery := &models.GetAuthInfoQuery{}
		authQuery.AuthModule = authModule
		authQuery.AuthId = authId
		authQuery.UserId = userID
		err := s.authInfoStore.GetAuthInfo(ctx, authQuery)
		if err != nil {
			return nil, err
		}

		return authQuery.Result, nil
	}
	return nil, nil
}

func (s *Implementation) LookupAndUpdate(ctx context.Context, query *models.GetUserByAuthInfoQuery) (*user.User, error) {
	// 1. LookupAndFix = auth info, user, error
	// TODO: Not a big fan of the fact that we are deleting auth info here, might want to move that
	foundUser, usr, authInfo, err := s.LookupAndFix(ctx, query)
	if err != nil && !errors.Is(err, user.ErrUserNotFound) {
		return nil, err
	}

	// 2. FindByUserDetails
	if !foundUser {
		usr, err = s.LookupByOneOf(ctx, &query.UserLookupParams)
		if err != nil {
			return nil, err
		}
	}

	if err := s.UserProtectionService.AllowUserMapping(usr, query.AuthModule); err != nil {
		return nil, err
	}

	// Special case for generic oauth duplicates
	ai, err := s.GenericOAuthLookup(ctx, query.AuthModule, query.AuthId, usr.ID)
	if !errors.Is(err, user.ErrUserNotFound) {
		if err != nil {
			return nil, err
		}
	}
	if ai != nil {
		authInfo = ai
	}

	if query.AuthModule != "" {
		if authInfo == nil {
			cmd := &models.SetAuthInfoCommand{
				UserId:     usr.ID,
				AuthModule: query.AuthModule,
				AuthId:     query.AuthId,
			}
			if err := s.authInfoStore.SetAuthInfo(ctx, cmd); err != nil {
				return nil, err
			}
		} else {
			if err := s.authInfoStore.UpdateAuthInfoDate(ctx, authInfo); err != nil {
				return nil, err
			}
		}
	}

	return usr, nil
}

func (s *Implementation) GetAuthInfo(ctx context.Context, query *models.GetAuthInfoQuery) error {
	return s.authInfoStore.GetAuthInfo(ctx, query)
}

func (s *Implementation) UpdateAuthInfo(ctx context.Context, cmd *models.UpdateAuthInfoCommand) error {
	return s.authInfoStore.UpdateAuthInfo(ctx, cmd)
}

func (s *Implementation) SetAuthInfo(ctx context.Context, cmd *models.SetAuthInfoCommand) error {
	return s.authInfoStore.SetAuthInfo(ctx, cmd)
}

func (s *Implementation) GetExternalUserInfoByLogin(ctx context.Context, query *models.GetExternalUserInfoByLoginQuery) error {
	return s.authInfoStore.GetExternalUserInfoByLogin(ctx, query)
}

func (s *Implementation) Run(ctx context.Context) error {
	s.logger.Debug("Started AuthInfo Metrics collection service")
	return s.authInfoStore.RunMetricsCollection(ctx)
}

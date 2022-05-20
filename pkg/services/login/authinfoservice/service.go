package authinfoservice

import (
	"context"
	"errors"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/login"
)

const genericOAuthModule = "oauth_generic_oauth"

type Implementation struct {
	UserProtectionService login.UserProtectionService
	authInfoStore         login.Store
	logger                log.Logger
}

func ProvideAuthInfoService(userProtectionService login.UserProtectionService, authInfoStore login.Store) *Implementation {
	s := &Implementation{
		UserProtectionService: userProtectionService,
		authInfoStore:         authInfoStore,
		logger:                log.New("login.authinfo"),
	}

	return s
}

func (s *Implementation) LookupAndFix(ctx context.Context, query *models.GetUserByAuthInfoQuery) (bool, *models.User, *models.UserAuth, error) {
	authQuery := &models.GetAuthInfoQuery{}

	// Try to find the user by auth module and id first
	if query.AuthModule != "" && query.AuthId != "" {
		authQuery.AuthModule = query.AuthModule
		authQuery.AuthId = query.AuthId

		err := s.authInfoStore.GetAuthInfo(ctx, authQuery)
		if !errors.Is(err, models.ErrUserNotFound) {
			if err != nil {
				return false, nil, nil, err
			}

			// if user id was specified and doesn't match the user_auth entry, remove it
			if query.UserId != 0 && query.UserId != authQuery.Result.UserId {
				err := s.authInfoStore.DeleteAuthInfo(ctx, &models.DeleteAuthInfoCommand{
					UserAuth: authQuery.Result,
				})
				if err != nil {
					s.logger.Error("Error removing user_auth entry", "error", err)
				}

				return false, nil, nil, models.ErrUserNotFound
			} else {
				user, err := s.authInfoStore.GetUserById(ctx, authQuery.Result.UserId)
				if err != nil {
					if errors.Is(err, models.ErrUserNotFound) {
						// if the user has been deleted then remove the entry
						if errDel := s.authInfoStore.DeleteAuthInfo(ctx, &models.DeleteAuthInfoCommand{
							UserAuth: authQuery.Result,
						}); errDel != nil {
							s.logger.Error("Error removing user_auth entry", "error", errDel)
						}

						return false, nil, nil, models.ErrUserNotFound
					}

					return false, nil, nil, err
				}

				return true, user, authQuery.Result, nil
			}
		}
	}

	return false, nil, nil, models.ErrUserNotFound
}

func (s *Implementation) LookupByOneOf(ctx context.Context, userId int64, email string, login string) (*models.User, error) {
	var user *models.User
	var err error

	// If not found, try to find the user by id
	if userId != 0 {
		user, err = s.authInfoStore.GetUserById(ctx, userId)
		if err != nil && !errors.Is(err, models.ErrUserNotFound) {
			return nil, err
		}
	}

	// If not found, try to find the user by email address
	if user == nil && email != "" {
		user, err = s.authInfoStore.GetUserByEmail(ctx, email)
		if err != nil && !errors.Is(err, models.ErrUserNotFound) {
			return nil, err
		}
	}

	// If not found, try to find the user by login
	if user == nil && login != "" {
		user, err = s.authInfoStore.GetUserByLogin(ctx, login)
		if err != nil && !errors.Is(err, models.ErrUserNotFound) {
			return nil, err
		}
	}

	if user == nil {
		return nil, models.ErrUserNotFound
	}

	return user, nil
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

func (s *Implementation) LookupAndUpdate(ctx context.Context, query *models.GetUserByAuthInfoQuery) (*models.User, error) {
	// 1. LookupAndFix = auth info, user, error
	// TODO: Not a big fan of the fact that we are deleting auth info here, might want to move that
	foundUser, user, authInfo, err := s.LookupAndFix(ctx, query)
	if err != nil && !errors.Is(err, models.ErrUserNotFound) {
		return nil, err
	}

	// 2. FindByUserDetails
	if !foundUser {
		user, err = s.LookupByOneOf(ctx, query.UserId, query.Email, query.Login)
		if err != nil {
			return nil, err
		}
	}

	if err := s.UserProtectionService.AllowUserMapping(user, query.AuthModule); err != nil {
		return nil, err
	}

	// Special case for generic oauth duplicates
	ai, err := s.GenericOAuthLookup(ctx, query.AuthModule, query.AuthId, user.Id)
	if !errors.Is(err, models.ErrUserNotFound) {
		if err != nil {
			return nil, err
		}
	}
	if ai != nil {
		authInfo = ai
	}

	if authInfo == nil && query.AuthModule != "" {
		cmd := &models.SetAuthInfoCommand{
			UserId:     user.Id,
			AuthModule: query.AuthModule,
			AuthId:     query.AuthId,
		}
		if err := s.authInfoStore.SetAuthInfo(ctx, cmd); err != nil {
			return nil, err
		}
	}

	return user, nil
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

package login

import (
	"context"
	"time"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/sqlstore"
)

var (
	maxInvalidLoginAttempts int64 = 5
	loginAttemptsWindow           = time.Minute * 5
)

var validateLoginAttempts = func(ctx context.Context, query *models.LoginUserQuery, store sqlstore.Store) error {
	if query.Cfg.DisableBruteForceLoginProtection {
		return nil
	}

	loginAttemptCountQuery := models.GetUserLoginAttemptCountQuery{
		Username: query.Username,
		Since:    time.Now().Add(-loginAttemptsWindow),
	}

	if err := store.GetUserLoginAttemptCount(ctx, &loginAttemptCountQuery); err != nil {
		return err
	}

	if loginAttemptCountQuery.Result >= maxInvalidLoginAttempts {
		return ErrTooManyLoginAttempts
	}

	return nil
}

var saveInvalidLoginAttempt = func(ctx context.Context, query *models.LoginUserQuery, store sqlstore.Store) error {
	if query.Cfg.DisableBruteForceLoginProtection {
		return nil
	}

	loginAttemptCommand := models.CreateLoginAttemptCommand{
		Username:  query.Username,
		IpAddress: query.IpAddress,
	}

	return store.CreateLoginAttempt(ctx, &loginAttemptCommand)
}

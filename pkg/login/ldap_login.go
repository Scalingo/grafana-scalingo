package login

import (
	"context"
	"errors"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/ldap"
	"github.com/grafana/grafana/pkg/services/login"
	"github.com/grafana/grafana/pkg/services/multildap"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util/errutil"
)

// getLDAPConfig gets LDAP config
var getLDAPConfig = multildap.GetConfig

// isLDAPEnabled checks if LDAP is enabled
var isLDAPEnabled = multildap.IsEnabled

// newLDAP creates multiple LDAP instance
var newLDAP = multildap.New

// logger for the LDAP auth
var ldapLogger = log.New("login.ldap")

// loginUsingLDAP logs in user using LDAP. It returns whether LDAP is enabled and optional error and query arg will be
// populated with the logged in user if successful.
var loginUsingLDAP = func(ctx context.Context, query *models.LoginUserQuery, loginService login.Service) (bool, error) {
	enabled := isLDAPEnabled()

	if !enabled {
		return false, nil
	}

	config, err := getLDAPConfig(query.Cfg)
	if err != nil {
		return true, errutil.Wrap("Failed to get LDAP config", err)
	}

	externalUser, err := newLDAP(config.Servers).Login(query)
	if err != nil {
		if errors.Is(err, ldap.ErrCouldNotFindUser) {
			// Ignore the error since user might not be present anyway
			if err := loginService.DisableExternalUser(ctx, query.Username); err != nil {
				ldapLogger.Debug("Failed to disable external user", "err", err)
			}

			return true, ldap.ErrInvalidCredentials
		}

		return true, err
	}

	upsert := &models.UpsertUserCommand{
		ReqContext:    query.ReqContext,
		ExternalUser:  externalUser,
		SignupAllowed: setting.LDAPAllowSignup,
	}
	err = loginService.UpsertUser(ctx, upsert)
	if err != nil {
		return true, err
	}
	query.User = upsert.Result

	return true, nil
}

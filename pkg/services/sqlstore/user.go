package sqlstore

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/events"
	"github.com/grafana/grafana/pkg/models"
	ac "github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

func getOrgIdForNewUser(sess *DBSession, cmd models.CreateUserCommand) (int64, error) {
	if cmd.SkipOrgSetup {
		return -1, nil
	}

	if setting.AutoAssignOrg && cmd.OrgId != 0 {
		err := verifyExistingOrg(sess, cmd.OrgId)
		if err != nil {
			return -1, err
		}
		return cmd.OrgId, nil
	}

	orgName := cmd.OrgName
	if len(orgName) == 0 {
		orgName = util.StringsFallback2(cmd.Email, cmd.Login)
	}

	return getOrCreateOrg(sess, orgName)
}

type userCreationArgs struct {
	Login          string
	Email          string
	Name           string
	Company        string
	Password       string
	IsAdmin        bool
	IsDisabled     bool
	EmailVerified  bool
	OrgID          int64
	OrgName        string
	DefaultOrgRole string
}

func (ss *SQLStore) getOrgIDForNewUser(sess *DBSession, args userCreationArgs) (int64, error) {
	if ss.Cfg.AutoAssignOrg && args.OrgID != 0 {
		if err := verifyExistingOrg(sess, args.OrgID); err != nil {
			return -1, err
		}
		return args.OrgID, nil
	}

	orgName := args.OrgName
	if orgName == "" {
		orgName = util.StringsFallback2(args.Email, args.Login)
	}

	return ss.getOrCreateOrg(sess, orgName)
}

// createUser creates a user in the database
func (ss *SQLStore) createUser(ctx context.Context, sess *DBSession, args userCreationArgs, skipOrgSetup bool) (models.User, error) {
	var user models.User
	var orgID int64 = -1
	if !skipOrgSetup {
		var err error
		orgID, err = ss.getOrgIDForNewUser(sess, args)
		if err != nil {
			return user, err
		}
	}

	if args.Email == "" {
		args.Email = args.Login
	}

	exists, err := sess.Where("email=? OR login=?", args.Email, args.Login).Get(&models.User{})
	if err != nil {
		return user, err
	}
	if exists {
		return user, models.ErrUserAlreadyExists
	}

	// create user
	user = models.User{
		Email:            args.Email,
		Name:             args.Name,
		Login:            args.Login,
		Company:          args.Company,
		IsAdmin:          args.IsAdmin,
		IsDisabled:       args.IsDisabled,
		OrgId:            orgID,
		EmailVerified:    args.EmailVerified,
		Created:          time.Now(),
		Updated:          time.Now(),
		LastSeenAt:       time.Now().AddDate(-10, 0, 0),
		IsServiceAccount: false,
	}

	salt, err := util.GetRandomString(10)
	if err != nil {
		return user, err
	}
	user.Salt = salt
	rands, err := util.GetRandomString(10)
	if err != nil {
		return user, err
	}
	user.Rands = rands

	if len(args.Password) > 0 {
		encodedPassword, err := util.EncodePassword(args.Password, user.Salt)
		if err != nil {
			return user, err
		}
		user.Password = encodedPassword
	}

	sess.UseBool("is_admin")

	if _, err := sess.Insert(&user); err != nil {
		return user, err
	}

	sess.publishAfterCommit(&events.UserCreated{
		Timestamp: user.Created,
		Id:        user.Id,
		Name:      user.Name,
		Login:     user.Login,
		Email:     user.Email,
	})

	// create org user link
	if !skipOrgSetup {
		orgUser := models.OrgUser{
			OrgId:   orgID,
			UserId:  user.Id,
			Role:    models.ROLE_ADMIN,
			Created: time.Now(),
			Updated: time.Now(),
		}

		if ss.Cfg.AutoAssignOrg && !user.IsAdmin {
			if len(args.DefaultOrgRole) > 0 {
				orgUser.Role = models.RoleType(args.DefaultOrgRole)
			} else {
				orgUser.Role = models.RoleType(ss.Cfg.AutoAssignOrgRole)
			}
		}

		if _, err = sess.Insert(&orgUser); err != nil {
			return user, err
		}
	}

	return user, nil
}

func (ss *SQLStore) CreateUser(ctx context.Context, cmd models.CreateUserCommand) (*models.User, error) {
	var user *models.User
	err := ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		orgId, err := getOrgIdForNewUser(sess, cmd)
		if err != nil {
			return err
		}

		if cmd.Email == "" {
			cmd.Email = cmd.Login
		}

		exists, err := sess.Where("email=? OR login=?", cmd.Email, cmd.Login).Get(&models.User{})
		if err != nil {
			return err
		}
		if exists {
			return models.ErrUserAlreadyExists
		}

		// create user
		user = &models.User{
			Email:            cmd.Email,
			Name:             cmd.Name,
			Login:            cmd.Login,
			Company:          cmd.Company,
			IsAdmin:          cmd.IsAdmin,
			IsDisabled:       cmd.IsDisabled,
			OrgId:            orgId,
			EmailVerified:    cmd.EmailVerified,
			Created:          time.Now(),
			Updated:          time.Now(),
			LastSeenAt:       time.Now().AddDate(-10, 0, 0),
			IsServiceAccount: cmd.IsServiceAccount,
		}

		salt, err := util.GetRandomString(10)
		if err != nil {
			return err
		}
		user.Salt = salt
		rands, err := util.GetRandomString(10)
		if err != nil {
			return err
		}
		user.Rands = rands

		if len(cmd.Password) > 0 {
			encodedPassword, err := util.EncodePassword(cmd.Password, user.Salt)
			if err != nil {
				return err
			}
			user.Password = encodedPassword
		}

		sess.UseBool("is_admin")

		if _, err := sess.Insert(user); err != nil {
			return err
		}

		sess.publishAfterCommit(&events.UserCreated{
			Timestamp: user.Created,
			Id:        user.Id,
			Name:      user.Name,
			Login:     user.Login,
			Email:     user.Email,
		})

		// create org user link
		if !cmd.SkipOrgSetup {
			orgUser := models.OrgUser{
				OrgId:   orgId,
				UserId:  user.Id,
				Role:    models.ROLE_ADMIN,
				Created: time.Now(),
				Updated: time.Now(),
			}

			if setting.AutoAssignOrg && !user.IsAdmin {
				if len(cmd.DefaultOrgRole) > 0 {
					orgUser.Role = models.RoleType(cmd.DefaultOrgRole)
				} else {
					orgUser.Role = models.RoleType(setting.AutoAssignOrgRole)
				}
			}

			if _, err = sess.Insert(&orgUser); err != nil {
				return err
			}
		}

		return nil
	})

	return user, err
}

func notServiceAccountFilter(ss *SQLStore) string {
	return fmt.Sprintf("%s.is_service_account = %s",
		ss.Dialect.Quote("user"),
		ss.Dialect.BooleanStr(false))
}

func (ss *SQLStore) GetUserById(ctx context.Context, query *models.GetUserByIdQuery) error {
	return ss.WithDbSession(ctx, func(sess *DBSession) error {
		user := new(models.User)

		has, err := sess.ID(query.Id).
			Where(notServiceAccountFilter(ss)).
			Get(user)

		if err != nil {
			return err
		} else if !has {
			return models.ErrUserNotFound
		}

		query.Result = user

		return nil
	})
}

func (ss *SQLStore) GetUserByLogin(ctx context.Context, query *models.GetUserByLoginQuery) error {
	return ss.WithDbSession(ctx, func(sess *DBSession) error {
		if query.LoginOrEmail == "" {
			return models.ErrUserNotFound
		}

		// Try and find the user by login first.
		// It's not sufficient to assume that a LoginOrEmail with an "@" is an email.
		user := &models.User{Login: query.LoginOrEmail}
		has, err := sess.Where(notServiceAccountFilter(ss)).Get(user)

		if err != nil {
			return err
		}

		if !has && strings.Contains(query.LoginOrEmail, "@") {
			// If the user wasn't found, and it contains an "@" fallback to finding the
			// user by email.
			user = &models.User{Email: query.LoginOrEmail}
			has, err = sess.Get(user)
		}

		if err != nil {
			return err
		} else if !has {
			return models.ErrUserNotFound
		}

		query.Result = user

		return nil
	})
}

func (ss *SQLStore) GetUserByEmail(ctx context.Context, query *models.GetUserByEmailQuery) error {
	return ss.WithDbSession(ctx, func(sess *DBSession) error {
		if query.Email == "" {
			return models.ErrUserNotFound
		}

		user := &models.User{Email: query.Email}
		has, err := sess.Where(notServiceAccountFilter(ss)).Get(user)

		if err != nil {
			return err
		} else if !has {
			return models.ErrUserNotFound
		}

		query.Result = user

		return nil
	})
}

func (ss *SQLStore) UpdateUser(ctx context.Context, cmd *models.UpdateUserCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		user := models.User{
			Name:    cmd.Name,
			Email:   cmd.Email,
			Login:   cmd.Login,
			Theme:   cmd.Theme,
			Updated: time.Now(),
		}

		if _, err := sess.ID(cmd.UserId).Where(notServiceAccountFilter(ss)).Update(&user); err != nil {
			return err
		}

		sess.publishAfterCommit(&events.UserUpdated{
			Timestamp: user.Created,
			Id:        user.Id,
			Name:      user.Name,
			Login:     user.Login,
			Email:     user.Email,
		})

		return nil
	})
}

func (ss *SQLStore) ChangeUserPassword(ctx context.Context, cmd *models.ChangeUserPasswordCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		user := models.User{
			Password: cmd.NewPassword,
			Updated:  time.Now(),
		}

		_, err := sess.ID(cmd.UserId).Where(notServiceAccountFilter(ss)).Update(&user)
		return err
	})
}

func (ss *SQLStore) UpdateUserLastSeenAt(ctx context.Context, cmd *models.UpdateUserLastSeenAtCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		user := models.User{
			Id:         cmd.UserId,
			LastSeenAt: time.Now(),
		}

		_, err := sess.ID(cmd.UserId).Update(&user)
		return err
	})
}

func (ss *SQLStore) SetUsingOrg(ctx context.Context, cmd *models.SetUsingOrgCommand) error {
	getOrgsForUserCmd := &models.GetUserOrgListQuery{UserId: cmd.UserId}
	if err := ss.GetUserOrgList(ctx, getOrgsForUserCmd); err != nil {
		return err
	}

	valid := false
	for _, other := range getOrgsForUserCmd.Result {
		if other.OrgId == cmd.OrgId {
			valid = true
		}
	}
	if !valid {
		return fmt.Errorf("user does not belong to org")
	}

	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		return setUsingOrgInTransaction(sess, cmd.UserId, cmd.OrgId)
	})
}

func setUsingOrgInTransaction(sess *DBSession, userID int64, orgID int64) error {
	user := models.User{
		Id:    userID,
		OrgId: orgID,
	}

	_, err := sess.ID(userID).Update(&user)
	return err
}

func removeUserOrg(sess *DBSession, userID int64) error {
	user := models.User{
		Id:    userID,
		OrgId: 0,
	}

	_, err := sess.ID(userID).MustCols("org_id").Update(&user)
	return err
}

func (ss *SQLStore) GetUserProfile(ctx context.Context, query *models.GetUserProfileQuery) error {
	return ss.WithDbSession(ctx, func(sess *DBSession) error {
		var user models.User
		has, err := sess.ID(query.UserId).Where(notServiceAccountFilter(ss)).Get(&user)

		if err != nil {
			return err
		} else if !has {
			return models.ErrUserNotFound
		}

		query.Result = models.UserProfileDTO{
			Id:             user.Id,
			Name:           user.Name,
			Email:          user.Email,
			Login:          user.Login,
			Theme:          user.Theme,
			IsGrafanaAdmin: user.IsAdmin,
			IsDisabled:     user.IsDisabled,
			OrgId:          user.OrgId,
			UpdatedAt:      user.Updated,
			CreatedAt:      user.Created,
		}

		return err
	})
}

type byOrgName []*models.UserOrgDTO

// Len returns the length of an array of organisations.
func (o byOrgName) Len() int {
	return len(o)
}

// Swap swaps two indices of an array of organizations.
func (o byOrgName) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

// Less returns whether element i of an array of organizations is less than element j.
func (o byOrgName) Less(i, j int) bool {
	if strings.ToLower(o[i].Name) < strings.ToLower(o[j].Name) {
		return true
	}

	return o[i].Name < o[j].Name
}

func (ss *SQLStore) GetUserOrgList(ctx context.Context, query *models.GetUserOrgListQuery) error {
	return ss.WithDbSession(ctx, func(dbSess *DBSession) error {
		query.Result = make([]*models.UserOrgDTO, 0)
		sess := dbSess.Table("org_user")
		sess.Join("INNER", "org", "org_user.org_id=org.id")
		sess.Join("INNER", x.Dialect().Quote("user"), fmt.Sprintf("org_user.user_id=%s.id", x.Dialect().Quote("user")))
		sess.Where("org_user.user_id=?", query.UserId)
		sess.Where(notServiceAccountFilter(ss))
		sess.Cols("org.name", "org_user.role", "org_user.org_id")
		sess.OrderBy("org.name")
		err := sess.Find(&query.Result)
		sort.Sort(byOrgName(query.Result))
		return err
	})
}

func newSignedInUserCacheKey(orgID, userID int64) string {
	return fmt.Sprintf("signed-in-user-%d-%d", userID, orgID)
}

func (ss *SQLStore) GetSignedInUserWithCacheCtx(ctx context.Context, query *models.GetSignedInUserQuery) error {
	cacheKey := newSignedInUserCacheKey(query.OrgId, query.UserId)
	if cached, found := ss.CacheService.Get(cacheKey); found {
		cachedUser := cached.(models.SignedInUser)
		query.Result = &cachedUser
		return nil
	}

	err := ss.GetSignedInUser(ctx, query)
	if err != nil {
		return err
	}

	cacheKey = newSignedInUserCacheKey(query.Result.OrgId, query.UserId)
	ss.CacheService.Set(cacheKey, *query.Result, time.Second*5)
	return nil
}

func (ss *SQLStore) GetSignedInUser(ctx context.Context, query *models.GetSignedInUserQuery) error {
	return ss.WithDbSession(ctx, func(dbSess *DBSession) error {
		orgId := "u.org_id"
		if query.OrgId > 0 {
			orgId = strconv.FormatInt(query.OrgId, 10)
		}

		var rawSQL = `SELECT
		u.id                  as user_id,
		u.is_admin            as is_grafana_admin,
		u.email               as email,
		u.login               as login,
		u.name                as name,
		u.is_disabled         as is_disabled,
		u.help_flags1         as help_flags1,
		u.last_seen_at        as last_seen_at,
		(SELECT COUNT(*) FROM org_user where org_user.user_id = u.id) as org_count,
		user_auth.auth_module as external_auth_module,
		user_auth.auth_id     as external_auth_id,
		org.name              as org_name,
		org_user.role         as org_role,
		org.id                as org_id
		FROM ` + dialect.Quote("user") + ` as u
		LEFT OUTER JOIN user_auth on user_auth.user_id = u.id
		LEFT OUTER JOIN org_user on org_user.org_id = ` + orgId + ` and org_user.user_id = u.id
		LEFT OUTER JOIN org on org.id = org_user.org_id `

		sess := dbSess.Table("user")
		sess = sess.Context(ctx)
		switch {
		case query.UserId > 0:
			sess.SQL(rawSQL+"WHERE u.id=?", query.UserId)
		case query.Login != "":
			sess.SQL(rawSQL+"WHERE u.login=?", query.Login)
		case query.Email != "":
			sess.SQL(rawSQL+"WHERE u.email=?", query.Email)
		}

		var user models.SignedInUser
		has, err := sess.Get(&user)
		if err != nil {
			return err
		} else if !has {
			return models.ErrUserNotFound
		}

		if user.OrgRole == "" {
			user.OrgId = -1
			user.OrgName = "Org missing"
		}

		if user.ExternalAuthModule != "oauth_grafana_com" {
			user.ExternalAuthId = ""
		}

		getTeamsByUserQuery := &models.GetTeamsByUserQuery{OrgId: user.OrgId, UserId: user.UserId}
		err = ss.GetTeamsByUser(ctx, getTeamsByUserQuery)
		if err != nil {
			return err
		}

		user.Teams = make([]int64, len(getTeamsByUserQuery.Result))
		for i, t := range getTeamsByUserQuery.Result {
			user.Teams[i] = t.Id
		}

		query.Result = &user
		return err
	})
}

func (ss *SQLStore) SearchUsers(ctx context.Context, query *models.SearchUsersQuery) error {
	return ss.WithDbSession(ctx, func(dbSess *DBSession) error {
		query.Result = models.SearchUserQueryResult{
			Users: make([]*models.UserSearchHitDTO, 0),
		}

		queryWithWildcards := "%" + query.Query + "%"

		whereConditions := make([]string, 0)
		whereParams := make([]interface{}, 0)
		sess := dbSess.Table("user").Alias("u")

		whereConditions = append(whereConditions, "u.is_service_account = ?")
		whereParams = append(whereParams, dialect.BooleanStr(false))

		// Join with only most recent auth module
		joinCondition := `(
		SELECT id from user_auth
			WHERE user_auth.user_id = u.id
			ORDER BY user_auth.created DESC `
		joinCondition = "user_auth.id=" + joinCondition + dialect.Limit(1) + ")"
		sess.Join("LEFT", "user_auth", joinCondition)
		if query.OrgId > 0 {
			whereConditions = append(whereConditions, "org_id = ?")
			whereParams = append(whereParams, query.OrgId)
		}

		if query.Query != "" {
			whereConditions = append(whereConditions, "(email "+dialect.LikeStr()+" ? OR name "+dialect.LikeStr()+" ? OR login "+dialect.LikeStr()+" ?)")
			whereParams = append(whereParams, queryWithWildcards, queryWithWildcards, queryWithWildcards)
		}

		if query.IsDisabled != nil {
			whereConditions = append(whereConditions, "is_disabled = ?")
			whereParams = append(whereParams, query.IsDisabled)
		}

		if query.AuthModule != "" {
			whereConditions = append(whereConditions, `auth_module=?`)
			whereParams = append(whereParams, query.AuthModule)
		}

		if len(whereConditions) > 0 {
			sess.Where(strings.Join(whereConditions, " AND "), whereParams...)
		}

		for _, filter := range query.Filters {
			if jc := filter.JoinCondition(); jc != nil {
				sess.Join(jc.Operator, jc.Table, jc.Params)
			}
			if ic := filter.InCondition(); ic != nil {
				sess.In(ic.Condition, ic.Params)
			}
			if wc := filter.WhereCondition(); wc != nil {
				sess.Where(wc.Condition, wc.Params)
			}
		}

		if query.Limit > 0 {
			offset := query.Limit * (query.Page - 1)
			sess.Limit(query.Limit, offset)
		}

		sess.Cols("u.id", "u.email", "u.name", "u.login", "u.is_admin", "u.is_disabled", "u.last_seen_at", "user_auth.auth_module")
		sess.Asc("u.login", "u.email")
		if err := sess.Find(&query.Result.Users); err != nil {
			return err
		}

		// get total
		user := models.User{}
		countSess := dbSess.Table("user").Alias("u")

		// Join with user_auth table if users filtered by auth_module
		if query.AuthModule != "" {
			countSess.Join("LEFT", "user_auth", joinCondition)
		}

		if len(whereConditions) > 0 {
			countSess.Where(strings.Join(whereConditions, " AND "), whereParams...)
		}

		for _, filter := range query.Filters {
			if jc := filter.JoinCondition(); jc != nil {
				countSess.Join(jc.Operator, jc.Table, jc.Params)
			}
			if ic := filter.InCondition(); ic != nil {
				countSess.In(ic.Condition, ic.Params)
			}
			if wc := filter.WhereCondition(); wc != nil {
				countSess.Where(wc.Condition, wc.Params)
			}
		}

		count, err := countSess.Count(&user)
		query.Result.TotalCount = count

		for _, user := range query.Result.Users {
			user.LastSeenAtAge = util.GetAgeString(user.LastSeenAt)
		}

		return err
	})
}

func (ss *SQLStore) DisableUser(ctx context.Context, cmd *models.DisableUserCommand) error {
	return ss.WithDbSession(ctx, func(dbSess *DBSession) error {
		user := models.User{}
		sess := dbSess.Table("user")

		if has, err := sess.ID(cmd.UserId).Where(notServiceAccountFilter(ss)).Get(&user); err != nil {
			return err
		} else if !has {
			return models.ErrUserNotFound
		}

		user.IsDisabled = cmd.IsDisabled
		sess.UseBool("is_disabled")

		_, err := sess.ID(cmd.UserId).Update(&user)
		return err
	})
}

func (ss *SQLStore) BatchDisableUsers(ctx context.Context, cmd *models.BatchDisableUsersCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		userIds := cmd.UserIds

		if len(userIds) == 0 {
			return nil
		}

		user_id_params := strings.Repeat(",?", len(userIds)-1)
		disableSQL := "UPDATE " + dialect.Quote("user") + " SET is_disabled=? WHERE Id IN (?" + user_id_params + ")"

		disableParams := []interface{}{disableSQL, cmd.IsDisabled}
		for _, v := range userIds {
			disableParams = append(disableParams, v)
		}

		_, err := sess.Where(notServiceAccountFilter(ss)).Exec(disableParams...)
		return err
	})
}

func (ss *SQLStore) DeleteUser(ctx context.Context, cmd *models.DeleteUserCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		return deleteUserInTransaction(ss, sess, cmd)
	})
}

func deleteUserInTransaction(ss *SQLStore, sess *DBSession, cmd *models.DeleteUserCommand) error {
	// Check if user exists
	user := models.User{Id: cmd.UserId}
	has, err := sess.Where(notServiceAccountFilter(ss)).Get(&user)
	if err != nil {
		return err
	}
	if !has {
		return models.ErrUserNotFound
	}
	for _, sql := range UserDeletions() {
		_, err := sess.Exec(sql, cmd.UserId)
		if err != nil {
			return err
		}
	}

	return deleteUserAccessControl(sess, cmd.UserId)
}

func deleteUserAccessControl(sess *DBSession, userID int64) error {
	// Delete user role assignments
	if _, err := sess.Exec("DELETE FROM user_role WHERE user_id = ?", userID); err != nil {
		return err
	}

	// Delete permissions that are scoped to user
	if _, err := sess.Exec("DELETE FROM permission WHERE scope = ?", ac.Scope("users", "id", strconv.FormatInt(userID, 10))); err != nil {
		return err
	}

	var roleIDs []int64
	if err := sess.SQL("SELECT id FROM role WHERE name = ?", ac.ManagedUserRoleName(userID)).Find(&roleIDs); err != nil {
		return err
	}

	if len(roleIDs) == 0 {
		return nil
	}

	query := "DELETE FROM permission WHERE role_id IN(? " + strings.Repeat(",?", len(roleIDs)-1) + ")"
	args := make([]interface{}, 0, len(roleIDs)+1)
	args = append(args, query)
	for _, id := range roleIDs {
		args = append(args, id)
	}

	// Delete managed user permissions
	if _, err := sess.Exec(args...); err != nil {
		return err
	}

	// Delete managed user roles
	if _, err := sess.Exec("DELETE FROM role WHERE name = ?", ac.ManagedUserRoleName(userID)); err != nil {
		return err
	}

	return nil
}

func UserDeletions() []string {
	deletes := []string{
		"DELETE FROM star WHERE user_id = ?",
		"DELETE FROM " + dialect.Quote("user") + " WHERE id = ?",
		"DELETE FROM org_user WHERE user_id = ?",
		"DELETE FROM dashboard_acl WHERE user_id = ?",
		"DELETE FROM preferences WHERE user_id = ?",
		"DELETE FROM team_member WHERE user_id = ?",
		"DELETE FROM user_auth WHERE user_id = ?",
		"DELETE FROM user_auth_token WHERE user_id = ?",
		"DELETE FROM quota WHERE user_id = ?",
	}
	return deletes
}

// UpdateUserPermissions sets the user Server Admin flag
func (ss *SQLStore) UpdateUserPermissions(userID int64, isAdmin bool) error {
	return ss.WithTransactionalDbSession(context.Background(), func(sess *DBSession) error {
		var user models.User
		if _, err := sess.ID(userID).Where(notServiceAccountFilter(ss)).Get(&user); err != nil {
			return err
		}

		user.IsAdmin = isAdmin
		sess.UseBool("is_admin")

		_, err := sess.ID(user.Id).Update(&user)
		if err != nil {
			return err
		}

		// validate that after update there is at least one server admin
		if err := validateOneAdminLeft(sess); err != nil {
			return err
		}

		return nil
	})
}

func (ss *SQLStore) SetUserHelpFlag(ctx context.Context, cmd *models.SetUserHelpFlagCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		user := models.User{
			Id:         cmd.UserId,
			HelpFlags1: cmd.HelpFlags1,
			Updated:    time.Now(),
		}

		_, err := sess.ID(cmd.UserId).Cols("help_flags1").Update(&user)
		return err
	})
}

// validateOneAdminLeft validate that there is an admin user left
func validateOneAdminLeft(sess *DBSession) error {
	count, err := sess.Where("is_admin=?", true).Count(&models.User{})
	if err != nil {
		return err
	}

	if count == 0 {
		return models.ErrLastGrafanaAdmin
	}

	return nil
}

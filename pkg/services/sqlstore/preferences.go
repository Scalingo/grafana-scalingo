package sqlstore

import (
	"context"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/models"
)

func (ss *SQLStore) GetPreferencesWithDefaults(ctx context.Context, query *models.GetPreferencesWithDefaultsQuery) error {
	return ss.WithDbSession(ctx, func(dbSession *DBSession) error {
		params := make([]interface{}, 0)
		filter := ""

		if len(query.User.Teams) > 0 {
			filter = "(org_id=? AND team_id IN (?" + strings.Repeat(",?", len(query.User.Teams)-1) + ")) OR "
			params = append(params, query.User.OrgId)
			for _, v := range query.User.Teams {
				params = append(params, v)
			}
		}

		filter += "(org_id=? AND user_id=? AND team_id=0) OR (org_id=? AND team_id=0 AND user_id=0)"
		params = append(params, query.User.OrgId)
		params = append(params, query.User.UserId)
		params = append(params, query.User.OrgId)
		prefs := make([]*models.Preferences, 0)
		err := dbSession.Where(filter, params...).
			OrderBy("user_id ASC, team_id ASC").
			Find(&prefs)

		if err != nil {
			return err
		}

		res := &models.Preferences{
			Theme:           ss.Cfg.DefaultTheme,
			Timezone:        ss.Cfg.DateFormats.DefaultTimezone,
			WeekStart:       ss.Cfg.DateFormats.DefaultWeekStart,
			HomeDashboardId: 0,
			JsonData:        &models.PreferencesJsonData{},
		}

		for _, p := range prefs {
			if p.Theme != "" {
				res.Theme = p.Theme
			}
			if p.Timezone != "" {
				res.Timezone = p.Timezone
			}
			if p.WeekStart != "" {
				res.WeekStart = p.WeekStart
			}
			if p.HomeDashboardId != 0 {
				res.HomeDashboardId = p.HomeDashboardId
			}
			if p.JsonData != nil {
				res.JsonData = p.JsonData
			}
		}

		query.Result = res
		return nil
	})
}

func (ss *SQLStore) GetPreferences(ctx context.Context, query *models.GetPreferencesQuery) error {
	return ss.WithDbSession(ctx, func(sess *DBSession) error {
		var prefs models.Preferences
		exists, err := sess.Where("org_id=? AND user_id=? AND team_id=?", query.OrgId, query.UserId, query.TeamId).Get(&prefs)

		if err != nil {
			return err
		}

		if exists {
			query.Result = &prefs
		} else {
			query.Result = new(models.Preferences)
		}

		return nil
	})
}

func (ss *SQLStore) SavePreferences(ctx context.Context, cmd *models.SavePreferencesCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		var prefs models.Preferences
		exists, err := sess.Where("org_id=? AND user_id=? AND team_id=?", cmd.OrgId, cmd.UserId, cmd.TeamId).Get(&prefs)
		if err != nil {
			return err
		}

		if !exists {
			prefs = models.Preferences{
				UserId:          cmd.UserId,
				OrgId:           cmd.OrgId,
				TeamId:          cmd.TeamId,
				HomeDashboardId: cmd.HomeDashboardId,
				Timezone:        cmd.Timezone,
				WeekStart:       cmd.WeekStart,
				Theme:           cmd.Theme,
				Created:         time.Now(),
				Updated:         time.Now(),
				JsonData:        &models.PreferencesJsonData{},
			}

			if cmd.Navbar != nil {
				prefs.JsonData.Navbar = *cmd.Navbar
			}
			_, err = sess.Insert(&prefs)
			return err
		}
		// Wrap this in an if statement to maintain backwards compatibility
		if cmd.Navbar != nil {
			if prefs.JsonData == nil {
				prefs.JsonData = &models.PreferencesJsonData{}
			}
			if cmd.Navbar.SavedItems != nil {
				prefs.JsonData.Navbar.SavedItems = cmd.Navbar.SavedItems
			}
		}
		prefs.HomeDashboardId = cmd.HomeDashboardId
		prefs.Timezone = cmd.Timezone
		prefs.WeekStart = cmd.WeekStart
		prefs.Theme = cmd.Theme
		prefs.Updated = time.Now()
		prefs.Version += 1
		_, err = sess.ID(prefs.Id).AllCols().Update(&prefs)
		return err
	})
}

func (ss *SQLStore) PatchPreferences(ctx context.Context, cmd *models.PatchPreferencesCommand) error {
	return ss.WithTransactionalDbSession(ctx, func(sess *DBSession) error {
		var prefs models.Preferences
		exists, err := sess.Where("org_id=? AND user_id=? AND team_id=?", cmd.OrgId, cmd.UserId, cmd.TeamId).Get(&prefs)
		if err != nil {
			return err
		}

		if !exists {
			prefs = models.Preferences{
				UserId:   cmd.UserId,
				OrgId:    cmd.OrgId,
				TeamId:   cmd.TeamId,
				Created:  time.Now(),
				JsonData: &models.PreferencesJsonData{},
			}
		}

		if cmd.Navbar != nil {
			if prefs.JsonData == nil {
				prefs.JsonData = &models.PreferencesJsonData{}
			}
			if cmd.Navbar.SavedItems != nil {
				prefs.JsonData.Navbar.SavedItems = cmd.Navbar.SavedItems
			}
		}

		if cmd.HomeDashboardId != nil {
			prefs.HomeDashboardId = *cmd.HomeDashboardId
		}

		if cmd.Timezone != nil {
			prefs.Timezone = *cmd.Timezone
		}

		if cmd.WeekStart != nil {
			prefs.WeekStart = *cmd.WeekStart
		}

		if cmd.Theme != nil {
			prefs.Theme = *cmd.Theme
		}

		prefs.Updated = time.Now()
		prefs.Version += 1

		if exists {
			_, err = sess.ID(prefs.Id).AllCols().Update(&prefs)
		} else {
			_, err = sess.Insert(&prefs)
		}
		return err
	})
}

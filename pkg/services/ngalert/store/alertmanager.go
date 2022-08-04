package store

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	"xorm.io/builder"
	"xorm.io/core"

	"github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/sqlstore"
)

var (
	// ErrNoAlertmanagerConfiguration is an error for when no alertmanager configuration is found.
	ErrNoAlertmanagerConfiguration = fmt.Errorf("could not find an Alertmanager configuration")
	// ErrVersionLockedObjectNotFound is returned when an object is not
	// found using the current hash.
	ErrVersionLockedObjectNotFound = fmt.Errorf("could not find object using provided id and hash")
	// ConfigRecordsLimit defines the limit of how many alertmanager configuration versions
	// should be stored in the database for each organization including the current one.
	// Has to be > 0
	ConfigRecordsLimit int64 = 100
)

// GetLatestAlertmanagerConfiguration returns the lastest version of the alertmanager configuration.
// It returns ErrNoAlertmanagerConfiguration if no configuration is found.
func (st *DBstore) GetLatestAlertmanagerConfiguration(ctx context.Context, query *models.GetLatestAlertmanagerConfigurationQuery) error {
	return st.SQLStore.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		c := &models.AlertConfiguration{}
		// The ID is already an auto incremental column, using the ID as an order should guarantee the latest.
		ok, err := sess.Desc("id").Where("org_id = ?", query.OrgID).Limit(1).Get(c)
		if err != nil {
			return err
		}

		if !ok {
			return ErrNoAlertmanagerConfiguration
		}

		query.Result = c
		return nil
	})
}

// GetAllLatestAlertmanagerConfiguration returns the latest configuration of every organization
func (st *DBstore) GetAllLatestAlertmanagerConfiguration(ctx context.Context) ([]*models.AlertConfiguration, error) {
	var result []*models.AlertConfiguration
	err := st.SQLStore.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		condition := builder.In("id", builder.Select("MAX(id)").From("alert_configuration").GroupBy("org_id"))
		if err := sess.Table("alert_configuration").Where(condition).Find(&result); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SaveAlertmanagerConfiguration creates an alertmanager configuration.
func (st DBstore) SaveAlertmanagerConfiguration(ctx context.Context, cmd *models.SaveAlertmanagerConfigurationCmd) error {
	return st.SaveAlertmanagerConfigurationWithCallback(ctx, cmd, func() error { return nil })
}

type SaveCallback func() error

// SaveAlertmanagerConfigurationWithCallback creates an alertmanager configuration version and then executes a callback.
// If the callback results in error it rolls back the transaction.
func (st DBstore) SaveAlertmanagerConfigurationWithCallback(ctx context.Context, cmd *models.SaveAlertmanagerConfigurationCmd, callback SaveCallback) error {
	return st.SQLStore.WithTransactionalDbSession(ctx, func(sess *sqlstore.DBSession) error {
		config := models.AlertConfiguration{
			AlertmanagerConfiguration: cmd.AlertmanagerConfiguration,
			ConfigurationHash:         fmt.Sprintf("%x", md5.Sum([]byte(cmd.AlertmanagerConfiguration))),
			ConfigurationVersion:      cmd.ConfigurationVersion,
			Default:                   cmd.Default,
			OrgID:                     cmd.OrgID,
		}
		if _, err := sess.Insert(config); err != nil {
			return err
		}
		if _, err := st.deleteOldConfigurations(ctx, cmd.OrgID, ConfigRecordsLimit); err != nil {
			st.Logger.Warn("failed to delete old am configs", "org", cmd.OrgID, "err", err)
		}
		if err := callback(); err != nil {
			return err
		}

		return nil
	})
}

func (st *DBstore) UpdateAlertmanagerConfiguration(ctx context.Context, cmd *models.SaveAlertmanagerConfigurationCmd) error {
	return st.SQLStore.WithTransactionalDbSession(ctx, func(sess *sqlstore.DBSession) error {
		config := models.AlertConfiguration{
			AlertmanagerConfiguration: cmd.AlertmanagerConfiguration,
			ConfigurationHash:         fmt.Sprintf("%x", md5.Sum([]byte(cmd.AlertmanagerConfiguration))),
			ConfigurationVersion:      cmd.ConfigurationVersion,
			Default:                   cmd.Default,
			OrgID:                     cmd.OrgID,
			CreatedAt:                 time.Now().Unix(),
		}
		res, err := sess.Exec(fmt.Sprintf(getInsertQuery(st.SQLStore.Dialect.DriverName()), st.SQLStore.Dialect.Quote("default")),
			config.AlertmanagerConfiguration,
			config.ConfigurationHash,
			config.ConfigurationVersion,
			config.OrgID,
			config.CreatedAt,
			st.SQLStore.Dialect.BooleanStr(config.Default),
			cmd.OrgID,
			cmd.OrgID,
			cmd.FetchedConfigurationHash,
		)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrVersionLockedObjectNotFound
		}
		if _, err := st.deleteOldConfigurations(ctx, cmd.OrgID, ConfigRecordsLimit); err != nil {
			st.Logger.Warn("failed to delete old am configs", "org", cmd.OrgID, "err", err)
		}
		return err
	})
}

// getInsertQuery is used to determinate the insert query for the alertmanager config
// based on the provided sql driver. This is necesarry as such an advanced query
// is not supported by our ORM and we need to generate it manually for each SQL dialect.
// We introduced this as part of a bug fix as the old approach wasn't working.
// Rel: https://github.com/grafana/grafana/issues/51356
func getInsertQuery(driver string) string {
	switch driver {
	case core.MYSQL:
		return `
		INSERT INTO alert_configuration
		(alertmanager_configuration, configuration_hash, configuration_version, org_id, created_at, %s) 
		SELECT T.* FROM (SELECT ? AS alertmanager_configuration,? AS configuration_hash,? AS configuration_version,? AS org_id,? AS created_at,? AS 'default') AS T
		WHERE
		EXISTS (
			SELECT 1 
			FROM alert_configuration 
			WHERE 
				org_id = ? 
			AND 
				id = (SELECT MAX(id) FROM alert_configuration WHERE org_id = ?) 
			AND 
				configuration_hash = ?
		)`
	case core.POSTGRES:
		return `
		INSERT INTO alert_configuration
		(alertmanager_configuration, configuration_hash, configuration_version, org_id, created_at, %s) 
		SELECT T.* FROM (VALUES($1,$2,$3,$4::bigint,$5::integer,$6::boolean)) AS T
		WHERE
		EXISTS (
			SELECT 1 
			FROM alert_configuration 
			WHERE 
				org_id = $7 
			AND 
				id = (SELECT MAX(id) FROM alert_configuration WHERE org_id = $8::bigint) 
			AND 
				configuration_hash = $9
		)`
	case core.SQLITE:
		return `
		INSERT INTO alert_configuration
		(alertmanager_configuration, configuration_hash, configuration_version, org_id, created_at, %s) 
		SELECT T.* FROM (VALUES(?,?,?,?,?,?)) AS T
		WHERE
		EXISTS (
			SELECT 1 
			FROM alert_configuration 
			WHERE 
				org_id = ? 
			AND 
				id = (SELECT MAX(id) FROM alert_configuration WHERE org_id = ?) 
			AND 
				configuration_hash = ?
		)`
	default:
		// SQLite version
		return `
		INSERT INTO alert_configuration
		(alertmanager_configuration, configuration_hash, configuration_version, org_id, created_at, %s) 
		SELECT T.* FROM (VALUES(?,?,?,?,?,?)) AS T
		WHERE
		EXISTS (
			SELECT 1 
			FROM alert_configuration 
			WHERE 
				org_id = ? 
			AND 
				id = (SELECT MAX(id) FROM alert_configuration WHERE org_id = ?) 
			AND 
				configuration_hash = ?
		)`
	}
}

func (st *DBstore) deleteOldConfigurations(ctx context.Context, orgID, limit int64) (int64, error) {
	if limit < 1 {
		return 0, fmt.Errorf("failed to delete old configurations: limit is set to '%d' but needs to be > 0", limit)
	}
	var affactedRows int64
	err := st.SQLStore.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		res, err := sess.Exec(`
			DELETE FROM 
				alert_configuration 
			WHERE
				org_id = ?
			AND 
				id NOT IN (
					SELECT T.* FROM (
						SELECT id 
						FROM alert_configuration 
						WHERE org_id = ? ORDER BY id DESC LIMIT ?
					)AS T
				)
		`, orgID, orgID, limit)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		affactedRows = rows
		if affactedRows > 0 {
			st.Logger.Info("deleted old alert_configuration(s)", "org", orgID, "limit", limit, "delete_count", affactedRows)
		}
		return nil
	})
	return affactedRows, err
}

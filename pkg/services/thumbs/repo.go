package thumbs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/sqlstore"
)

func newThumbnailRepo(store *sqlstore.SQLStore) thumbnailRepo {
	repo := &sqlThumbnailRepository{
		store: store,
		log:   log.New("thumbnails_repo"),
	}
	return repo
}

type sqlThumbnailRepository struct {
	store *sqlstore.SQLStore
	log   log.Logger
}

func (r *sqlThumbnailRepository) saveFromFile(ctx context.Context, filePath string, meta models.DashboardThumbnailMeta, dashboardVersion int) (int64, error) {
	// the filePath variable is never set by the user. it refers to a temporary file created either in
	//   1. thumbs/service.go, when user uploads a thumbnail
	//   2. the rendering service, when image-renderer returns a screenshot

	if !filepath.IsAbs(filePath) {
		r.log.Error("Received relative path", "dashboardUID", meta.DashboardUID, "err", filePath)
		return 0, errors.New("relative paths are not supported")
	}

	content, err := os.ReadFile(filepath.Clean(filePath))

	if err != nil {
		r.log.Error("error reading file", "dashboardUID", meta.DashboardUID, "err", err)
		return 0, err
	}

	return r.saveFromBytes(ctx, content, getMimeType(filePath), meta, dashboardVersion)
}

func getMimeType(filePath string) string {
	if strings.HasSuffix(filePath, ".webp") {
		return "image/webp"
	}

	return "image/png"
}

func (r *sqlThumbnailRepository) saveFromBytes(ctx context.Context, content []byte, mimeType string, meta models.DashboardThumbnailMeta, dashboardVersion int) (int64, error) {
	cmd := &models.SaveDashboardThumbnailCommand{
		DashboardThumbnailMeta: meta,
		Image:                  content,
		MimeType:               mimeType,
		DashboardVersion:       dashboardVersion,
	}

	_, err := r.store.SaveThumbnail(ctx, cmd)
	if err != nil {
		r.log.Error("Error saving to the db", "dashboardUID", meta.DashboardUID, "err", err)
		return 0, err
	}

	return cmd.Result.Id, nil
}

func (r *sqlThumbnailRepository) updateThumbnailState(ctx context.Context, state models.ThumbnailState, meta models.DashboardThumbnailMeta) error {
	return r.store.UpdateThumbnailState(ctx, &models.UpdateThumbnailStateCommand{
		State:                  state,
		DashboardThumbnailMeta: meta,
	})
}

func (r *sqlThumbnailRepository) getThumbnail(ctx context.Context, meta models.DashboardThumbnailMeta) (*models.DashboardThumbnail, error) {
	query := &models.GetDashboardThumbnailCommand{
		DashboardThumbnailMeta: meta,
	}
	return r.store.GetThumbnail(ctx, query)
}

func (r *sqlThumbnailRepository) findDashboardsWithStaleThumbnails(ctx context.Context, theme models.Theme, kind models.ThumbnailKind) ([]*models.DashboardWithStaleThumbnail, error) {
	return r.store.FindDashboardsWithStaleThumbnails(ctx, &models.FindDashboardsWithStaleThumbnailsCommand{
		IncludeManuallyUploadedThumbnails: false,
		Theme:                             theme,
		Kind:                              kind,
	})
}

func (r *sqlThumbnailRepository) doThumbnailsExist(ctx context.Context) (bool, error) {
	cmd := &models.FindDashboardThumbnailCountCommand{}
	count, err := r.store.FindThumbnailCount(ctx, cmd)
	if err != nil {
		r.log.Error("Error finding thumbnails", "err", err)
		return false, err
	}
	return count > 0, err
}

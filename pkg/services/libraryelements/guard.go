package libraryelements

import (
	"context"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/guardian"
)

func isGeneralFolder(folderID int64) bool {
	return folderID == 0
}

func (l *LibraryElementService) requireSupportedElementKind(kindAsInt int64) error {
	kind := models.LibraryElementKind(kindAsInt)
	switch kind {
	case models.PanelElement:
		return nil
	case models.VariableElement:
		return nil
	default:
		return errLibraryElementUnSupportedElementKind
	}
}

func (l *LibraryElementService) requirePermissionsOnFolder(ctx context.Context, user *models.SignedInUser, folderID int64) error {
	if isGeneralFolder(folderID) && user.HasRole(models.ROLE_EDITOR) {
		return nil
	}

	if isGeneralFolder(folderID) && user.HasRole(models.ROLE_VIEWER) {
		return models.ErrFolderAccessDenied
	}
	folder, err := l.folderService.GetFolderByID(ctx, user, folderID, user.OrgId)
	if err != nil {
		return err
	}

	g := guardian.New(ctx, folder.Id, user.OrgId, user)

	canEdit, err := g.CanEdit()
	if err != nil {
		return err
	}
	if !canEdit {
		return models.ErrFolderAccessDenied
	}

	return nil
}

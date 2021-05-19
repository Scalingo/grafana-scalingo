package librarypanels

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestCreateLibraryPanel(t *testing.T) {
	scenarioWithLibraryPanel(t, "When an admin tries to create a library panel that already exists, it should fail",
		func(t *testing.T, sc scenarioContext) {
			command := getCreateCommand(sc.folder.Id, "Text - Library Panel")
			resp := sc.service.createHandler(sc.reqContext, command)
			require.Equal(t, 400, resp.Status())
		})

	scenarioWithLibraryPanel(t, "When an admin tries to create a library panel that does not exists, it should succeed",
		func(t *testing.T, sc scenarioContext) {
			var expected = libraryPanelResult{
				Result: libraryPanel{
					ID:          1,
					OrgID:       1,
					FolderID:    1,
					UID:         sc.initialResult.Result.UID,
					Name:        "Text - Library Panel",
					Type:        "text",
					Description: "A description",
					Model: map[string]interface{}{
						"datasource":  "${DS_GDEV-TESTDATA}",
						"description": "A description",
						"id":          float64(1),
						"title":       "Text - Library Panel",
						"type":        "text",
					},
					Version: 1,
					Meta: LibraryPanelDTOMeta{
						CanEdit:             true,
						ConnectedDashboards: 0,
						Created:             sc.initialResult.Result.Meta.Created,
						Updated:             sc.initialResult.Result.Meta.Updated,
						CreatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "signed_in_user",
							AvatarUrl: "/avatar/37524e1eb8b3e32850b57db0a19af93b",
						},
						UpdatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "signed_in_user",
							AvatarUrl: "/avatar/37524e1eb8b3e32850b57db0a19af93b",
						},
					},
				},
			}
			if diff := cmp.Diff(expected, sc.initialResult, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})

	testScenario(t, "When an admin tries to create a library panel where name and panel title differ, it should update panel title",
		func(t *testing.T, sc scenarioContext) {
			command := getCreateCommand(1, "Library Panel Name")
			resp := sc.service.createHandler(sc.reqContext, command)
			var result = validateAndUnMarshalResponse(t, resp)
			var expected = libraryPanelResult{
				Result: libraryPanel{
					ID:          1,
					OrgID:       1,
					FolderID:    1,
					UID:         result.Result.UID,
					Name:        "Library Panel Name",
					Type:        "text",
					Description: "A description",
					Model: map[string]interface{}{
						"datasource":  "${DS_GDEV-TESTDATA}",
						"description": "A description",
						"id":          float64(1),
						"title":       "Library Panel Name",
						"type":        "text",
					},
					Version: 1,
					Meta: LibraryPanelDTOMeta{
						CanEdit:             true,
						ConnectedDashboards: 0,
						Created:             result.Result.Meta.Created,
						Updated:             result.Result.Meta.Updated,
						CreatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "signed_in_user",
							AvatarUrl: "/avatar/37524e1eb8b3e32850b57db0a19af93b",
						},
						UpdatedBy: LibraryPanelDTOMetaUser{
							ID:        1,
							Name:      "signed_in_user",
							AvatarUrl: "/avatar/37524e1eb8b3e32850b57db0a19af93b",
						},
					},
				},
			}
			if diff := cmp.Diff(expected, result, getCompareOptions()...); diff != "" {
				t.Fatalf("Result mismatch (-want +got):\n%s", diff)
			}
		})
}

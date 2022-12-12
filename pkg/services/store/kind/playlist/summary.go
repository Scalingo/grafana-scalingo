package playlist

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana/pkg/kinds/playlist"
	"github.com/grafana/grafana/pkg/models"
)

func GetObjectKindInfo() models.ObjectKindInfo {
	return models.ObjectKindInfo{
		ID:          models.StandardKindPlaylist,
		Name:        "Playlist",
		Description: "Cycle though a collection of dashboards automatically",
	}
}

func GetObjectSummaryBuilder() models.ObjectSummaryBuilder {
	return summaryBuilder
}

func summaryBuilder(ctx context.Context, uid string, body []byte) (*models.ObjectSummary, []byte, error) {
	obj := &playlist.Playlist{}
	err := json.Unmarshal(body, obj)
	if err != nil {
		return nil, nil, err // unable to read object
	}

	// TODO: fix model so this is not possible
	if obj.Items == nil {
		temp := make([]playlist.PlaylistItem, 0)
		obj.Items = &temp
	}

	obj.Uid = uid // make sure they are consistent
	summary := &models.ObjectSummary{
		UID:         uid,
		Name:        obj.Name,
		Description: fmt.Sprintf("%d items, refreshed every %s", len(*obj.Items), obj.Interval),
	}

	for _, item := range *obj.Items {
		switch item.Type {
		case playlist.PlaylistItemTypeDashboardByUid:
			summary.References = append(summary.References, &models.ObjectExternalReference{
				Kind: "dashboard",
				UID:  item.Value,
			})

		case playlist.PlaylistItemTypeDashboardByTag:
			if summary.Labels == nil {
				summary.Labels = make(map[string]string, 0)
			}
			summary.Labels[item.Value] = ""

		case playlist.PlaylistItemTypeDashboardById:
			// obviously insufficient long term... but good to have an example :)
			summary.Error = &models.ObjectErrorInfo{
				Message: "Playlist uses deprecated internal id system",
			}
		}
	}

	out, err := json.MarshalIndent(obj, "", "  ")
	return summary, out, err
}

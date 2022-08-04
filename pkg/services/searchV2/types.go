package searchV2

import (
	"context"

	"github.com/grafana/grafana/pkg/registry"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type FacetField struct {
	Field string `json:"field"`
	Limit int    `json:"limit,omitempty"` // explicit page size
}

type DashboardQuery struct {
	Query        string       `json:"query"`
	Location     string       `json:"location,omitempty"` // parent folder ID
	Sort         string       `json:"sort,omitempty"`     // field ASC/DESC
	Datasource   string       `json:"ds_uid,omitempty"`   // "datasource" collides with the JSON value at the same leel :()
	Tags         []string     `json:"tags,omitempty"`
	Kind         []string     `json:"kind,omitempty"`
	UIDs         []string     `json:"uid,omitempty"`
	Explain      bool         `json:"explain,omitempty"` // adds details on why document matched
	Facet        []FacetField `json:"facet,omitempty"`
	SkipLocation bool         `json:"skipLocation,omitempty"`
	AccessInfo   bool         `json:"accessInfo,omitempty"` // adds field for access control
	HasPreview   string       `json:"hasPreview,omitempty"` // the light|dark theme
	Limit        int          `json:"limit,omitempty"`      // explicit page size
	From         int          `json:"from,omitempty"`       // for paging
}

type SearchService interface {
	registry.BackgroundService
	DoDashboardQuery(ctx context.Context, user *backend.User, orgId int64, query DashboardQuery) *backend.DataResponse
	RegisterDashboardIndexExtender(ext DashboardIndexExtender)
}

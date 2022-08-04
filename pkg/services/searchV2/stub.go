package searchV2

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

type stubSearchService struct {
}

func NewStubSearchService() SearchService {
	return &stubSearchService{}
}

func (s *stubSearchService) DoDashboardQuery(ctx context.Context, user *backend.User, orgId int64, query DashboardQuery) *backend.DataResponse {
	rsp := &backend.DataResponse{}

	// dashboards
	fid := data.NewFieldFromFieldType(data.FieldTypeInt64, 0)
	uid := data.NewFieldFromFieldType(data.FieldTypeString, 0)

	fid.Append(int64(2))
	uid.Append("hello")

	rsp.Frames = append(rsp.Frames, data.NewFrame("dasboards", fid, uid))

	return rsp
}

func (s *stubSearchService) RegisterDashboardIndexExtender(ext DashboardIndexExtender) {
	// noop
}

func (s *stubSearchService) Run(_ context.Context) error {
	return nil
}

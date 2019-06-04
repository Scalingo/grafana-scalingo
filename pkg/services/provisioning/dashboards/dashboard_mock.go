package dashboards

import "context"

type Calls struct {
	Provision                  []interface{}
	PollChanges                []interface{}
	GetProvisionerResolvedPath []interface{}
}

type DashboardProvisionerMock struct {
	Calls                          *Calls
	ProvisionFunc                  func() error
	PollChangesFunc                func(ctx context.Context)
	GetProvisionerResolvedPathFunc func(name string) string
}

func NewDashboardProvisionerMock() *DashboardProvisionerMock {
	return &DashboardProvisionerMock{
		Calls: &Calls{},
	}
}

func (dpm *DashboardProvisionerMock) Provision() error {
	dpm.Calls.Provision = append(dpm.Calls.Provision, nil)
	if dpm.ProvisionFunc != nil {
		return dpm.ProvisionFunc()
	} else {
		return nil
	}
}

func (dpm *DashboardProvisionerMock) PollChanges(ctx context.Context) {
	dpm.Calls.PollChanges = append(dpm.Calls.PollChanges, ctx)
	if dpm.PollChangesFunc != nil {
		dpm.PollChangesFunc(ctx)
	}
}

func (dpm *DashboardProvisionerMock) GetProvisionerResolvedPath(name string) string {
	dpm.Calls.PollChanges = append(dpm.Calls.GetProvisionerResolvedPath, name)
	if dpm.GetProvisionerResolvedPathFunc != nil {
		return dpm.GetProvisionerResolvedPathFunc(name)
	} else {
		return ""
	}
}

package folderimpl

import (
	"context"

	"github.com/grafana/grafana/pkg/services/folder"
)

type FakeStore struct {
	ExpectedFolders []*folder.Folder
	ExpectedFolder  *folder.Folder
	ExpectedError   error

	CreateCalled bool
	DeleteCalled bool
}

func NewFakeStore() *FakeStore {
	return &FakeStore{}
}

var _ store = (*FakeStore)(nil)

func (f *FakeStore) Create(ctx context.Context, cmd folder.CreateFolderCommand) (*folder.Folder, error) {
	f.CreateCalled = true
	return f.ExpectedFolder, f.ExpectedError
}

func (f *FakeStore) Delete(ctx context.Context, uid string, orgID int64) error {
	f.DeleteCalled = true
	return f.ExpectedError
}

func (f *FakeStore) Update(ctx context.Context, cmd folder.UpdateFolderCommand) (*folder.Folder, error) {
	return f.ExpectedFolder, f.ExpectedError
}

func (f *FakeStore) Move(ctx context.Context, cmd folder.MoveFolderCommand) error {
	return f.ExpectedError
}

func (f *FakeStore) Get(ctx context.Context, cmd folder.GetFolderQuery) (*folder.Folder, error) {
	return f.ExpectedFolder, f.ExpectedError
}

func (f *FakeStore) GetParents(ctx context.Context, cmd folder.GetParentsQuery) ([]*folder.Folder, error) {
	return f.ExpectedFolders, f.ExpectedError
}

func (f *FakeStore) GetChildren(ctx context.Context, cmd folder.GetTreeQuery) ([]*folder.Folder, error) {
	return f.ExpectedFolders, f.ExpectedError
}

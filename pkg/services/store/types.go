package store

import (
	"context"
	"encoding/json"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana/pkg/infra/filestorage"
	"github.com/grafana/grafana/pkg/models"
)

type WriteValueRequest struct {
	Path    string
	User    *models.SignedInUser
	Body    json.RawMessage `json:"body,omitempty"`
	Message string          `json:"message,omitempty"`
	Title   string          `json:"title,omitempty"`  // For PRs
	Action  string          `json:"action,omitempty"` // pr | save
}

type WriteValueResponse struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	URL     string `json:"url,omitempty"`
	Hash    string `json:"hash,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Pending bool   `json:"pending,omitempty"`
	Size    int64  `json:"size,omitempty"`
}

type storageTree interface {
	GetFile(ctx context.Context, path string) (*filestorage.File, error)
	ListFolder(ctx context.Context, path string) (*data.Frame, error)
}

//-------------------------------------------
// INTERNAL
//-------------------------------------------

type storageRuntime interface {
	Meta() RootStorageMeta

	Store() filestorage.FileStorage

	Sync() error

	// Different storage knows how to handle comments and tracking
	Write(ctx context.Context, cmd *WriteValueRequest) (*WriteValueResponse, error)
}

type baseStorageRuntime struct {
	meta  RootStorageMeta
	store filestorage.FileStorage
}

func (t *baseStorageRuntime) Meta() RootStorageMeta {
	return t.meta
}

func (t *baseStorageRuntime) Store() filestorage.FileStorage {
	return t.store
}

func (t *baseStorageRuntime) Sync() error {
	return nil
}

func (t *baseStorageRuntime) Write(ctx context.Context, cmd *WriteValueRequest) (*WriteValueResponse, error) {
	return &WriteValueResponse{
		Code:    500,
		Message: "unsupportted operation (base)",
	}, nil
}

func (t *baseStorageRuntime) setReadOnly(val bool) *baseStorageRuntime {
	t.meta.ReadOnly = val
	return t
}

func (t *baseStorageRuntime) setBuiltin(val bool) *baseStorageRuntime {
	t.meta.Builtin = val
	return t
}

type RootStorageMeta struct {
	ReadOnly bool          `json:"editable,omitempty"`
	Builtin  bool          `json:"builtin,omitempty"`
	Ready    bool          `json:"ready"` // can connect
	Notice   []data.Notice `json:"notice,omitempty"`

	Config RootStorageConfig `json:"config"`
}

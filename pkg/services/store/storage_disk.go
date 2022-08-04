package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana/pkg/infra/filestorage"
	"gocloud.dev/blob"
)

const rootStorageTypeDisk = "disk"

type rootStorageDisk struct {
	baseStorageRuntime

	settings *StorageLocalDiskConfig
}

func newDiskStorage(prefix string, name string, cfg *StorageLocalDiskConfig) *rootStorageDisk {
	if cfg == nil {
		cfg = &StorageLocalDiskConfig{}
	}

	meta := RootStorageMeta{
		Config: RootStorageConfig{
			Type:   rootStorageTypeDisk,
			Prefix: prefix,
			Name:   name,
			Disk:   cfg,
		},
	}
	if prefix == "" {
		meta.Notice = append(meta.Notice, data.Notice{
			Severity: data.NoticeSeverityError,
			Text:     "Missing prefix",
		})
	}
	if cfg.Path == "" {
		meta.Notice = append(meta.Notice, data.Notice{
			Severity: data.NoticeSeverityError,
			Text:     "Missing path configuration",
		})
	}
	s := &rootStorageDisk{}

	if meta.Notice == nil {
		path := fmt.Sprintf("file://%s", cfg.Path)
		bucket, err := blob.OpenBucket(context.Background(), path)
		if err != nil {
			grafanaStorageLogger.Warn("error loading storage", "prefix", prefix, "err", err)
			meta.Notice = append(meta.Notice, data.Notice{
				Severity: data.NoticeSeverityError,
				Text:     "Failed to initialize storage",
			})
		} else {
			s.store = filestorage.NewCdkBlobStorage(grafanaStorageLogger,
				bucket, "",
				filestorage.NewPathFilter(cfg.Roots, nil, nil, nil))

			meta.Ready = true // exists!
		}
	}

	s.meta = meta
	s.settings = cfg
	return s
}

func (s *rootStorageDisk) Sync() error {
	return nil // already in sync
}

// with local disk user metadata and messages are lost
func (s *rootStorageDisk) Write(ctx context.Context, cmd *WriteValueRequest) (*WriteValueResponse, error) {
	byteAray := []byte(cmd.Body)

	path := cmd.Path
	if !strings.HasPrefix(path, filestorage.Delimiter) {
		path = filestorage.Delimiter + path
	}
	err := s.store.Upsert(ctx, &filestorage.UpsertFileCommand{
		Path:     path,
		Contents: byteAray,
	})
	if err != nil {
		return nil, err
	}
	return &WriteValueResponse{Code: 200}, nil
}

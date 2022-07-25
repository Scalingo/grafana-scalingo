package filestorage

import (
	"context"
	"crypto/md5"
	"encoding/hex"

	// can ignore because we don't need a cryptographically secure hash function
	// sha1 low chance of collisions and better performance than sha256
	// nolint:gosec
	"crypto/sha1"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/services/sqlstore/migrator"
	"github.com/grafana/grafana/pkg/util/errutil"
)

type file struct {
	Path                 string    `xorm:"path"`
	PathHash             string    `xorm:"path_hash"`
	ParentFolderPathHash string    `xorm:"parent_folder_path_hash"`
	Contents             []byte    `xorm:"contents"`
	ETag                 string    `xorm:"etag"`
	CacheControl         string    `xorm:"cache_control"`
	ContentDisposition   string    `xorm:"content_disposition"`
	Updated              time.Time `xorm:"updated"`
	Created              time.Time `xorm:"created"`
	Size                 int64     `xorm:"size"`
	MimeType             string    `xorm:"mime_type"`
}

type fileMeta struct {
	PathHash string `xorm:"path_hash"`
	Key      string `xorm:"key"`
	Value    string `xorm:"value"`
}

type dbFileStorage struct {
	db  *sqlstore.SQLStore
	log log.Logger
}

func createPathHash(path string) (string, error) {
	hasher := sha1.New()
	if _, err := hasher.Write([]byte(strings.ToLower(path))); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func createContentsHash(contents []byte) string {
	hash := md5.Sum(contents)
	return hex.EncodeToString(hash[:])
}

func NewDbStorage(log log.Logger, db *sqlstore.SQLStore, filter PathFilter, rootFolder string) FileStorage {
	return newWrapper(log, &dbFileStorage{
		log: log,
		db:  db,
	}, filter, rootFolder)
}

func (s dbFileStorage) getProperties(sess *sqlstore.DBSession, pathHashes []string) (map[string]map[string]string, error) {
	attributesByPath := make(map[string]map[string]string)

	entities := make([]*fileMeta, 0)
	if err := sess.Table("file_meta").In("path_hash", pathHashes).Find(&entities); err != nil {
		return nil, err
	}

	for _, entity := range entities {
		if _, ok := attributesByPath[entity.PathHash]; !ok {
			attributesByPath[entity.PathHash] = make(map[string]string)
		}
		attributesByPath[entity.PathHash][entity.Key] = entity.Value
	}

	return attributesByPath, nil
}

func (s dbFileStorage) Get(ctx context.Context, filePath string) (*File, error) {
	var result *File

	pathHash, err := createPathHash(filePath)
	if err != nil {
		return nil, err
	}
	err = s.db.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		table := &file{}
		exists, err := sess.Table("file").Where("path_hash = ?", pathHash).Get(table)
		if !exists {
			return nil
		}

		var meta = make([]*fileMeta, 0)
		if err := sess.Table("file_meta").Where("path_hash = ?", pathHash).Find(&meta); err != nil {
			return err
		}

		var metaProperties = make(map[string]string, len(meta))

		for i := range meta {
			metaProperties[meta[i].Key] = meta[i].Value
		}

		contents := table.Contents
		if contents == nil {
			contents = make([]byte, 0)
		}

		result = &File{
			Contents: contents,
			FileMetadata: FileMetadata{
				Name:       getName(table.Path),
				FullPath:   table.Path,
				Created:    table.Created,
				Properties: metaProperties,
				Modified:   table.Updated,
				Size:       table.Size,
				MimeType:   table.MimeType,
			},
		}
		return err
	})

	return result, err
}

func (s dbFileStorage) Delete(ctx context.Context, filePath string) error {
	pathHash, err := createPathHash(filePath)
	if err != nil {
		return err
	}
	err = s.db.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		table := &file{}
		exists, innerErr := sess.Table("file").Where("path_hash = ?", pathHash).Get(table)
		if innerErr != nil {
			return innerErr
		}

		if !exists {
			return nil
		}

		number, innerErr := sess.Table("file").Where("path_hash = ?", pathHash).Delete(table)
		if innerErr != nil {
			return innerErr
		}
		s.log.Info("Deleted file", "path", filePath, "affectedRecords", number)

		metaTable := &fileMeta{}
		number, innerErr = sess.Table("file_meta").Where("path_hash = ?", pathHash).Delete(metaTable)
		if innerErr != nil {
			return innerErr
		}
		s.log.Info("Deleted metadata", "path", filePath, "affectedRecords", number)
		return innerErr
	})

	return err
}

func (s dbFileStorage) Upsert(ctx context.Context, cmd *UpsertFileCommand) error {
	now := time.Now()
	pathHash, err := createPathHash(cmd.Path)
	if err != nil {
		return err
	}

	err = s.db.WithTransactionalDbSession(ctx, func(sess *sqlstore.DBSession) error {
		existing := &file{}
		exists, err := sess.Table("file").Where("path_hash = ?", pathHash).Get(existing)
		if err != nil {
			return err
		}

		if exists {
			existing.Updated = now
			if cmd.Contents != nil {
				contents := cmd.Contents
				existing.Contents = contents
				existing.MimeType = cmd.MimeType
				existing.ETag = createContentsHash(contents)
				existing.ContentDisposition = cmd.ContentDisposition
				existing.CacheControl = cmd.CacheControl
				existing.Size = int64(len(contents))
			}

			_, err = sess.Where("path_hash = ?", pathHash).Update(existing)
			if err != nil {
				return err
			}
		} else {
			contentsToInsert := make([]byte, 0)
			if cmd.Contents != nil {
				contentsToInsert = cmd.Contents
			}

			parentFolderPath := getParentFolderPath(cmd.Path)
			parentFolderPathHash, err := createPathHash(parentFolderPath)
			if err != nil {
				return err
			}

			file := &file{
				Path:                 cmd.Path,
				PathHash:             pathHash,
				ParentFolderPathHash: parentFolderPathHash,
				Contents:             contentsToInsert,
				ContentDisposition:   cmd.ContentDisposition,
				CacheControl:         cmd.CacheControl,
				ETag:                 createContentsHash(contentsToInsert),
				MimeType:             cmd.MimeType,
				Size:                 int64(len(contentsToInsert)),
				Updated:              now,
				Created:              now,
			}
			if _, err = sess.Insert(file); err != nil {
				return err
			}
		}

		if len(cmd.Properties) != 0 {
			if err = upsertProperties(s.db.Dialect, sess, now, cmd, pathHash); err != nil {
				if rollbackErr := sess.Rollback(); rollbackErr != nil {
					s.log.Error("failed while rolling back upsert", "path", cmd.Path)
				}
				return err
			}
		}

		return err
	})

	return err
}

func upsertProperties(dialect migrator.Dialect, sess *sqlstore.DBSession, now time.Time, cmd *UpsertFileCommand, pathHash string) error {
	fileMeta := &fileMeta{}
	_, err := sess.Table("file_meta").Where("path_hash = ?", pathHash).Delete(fileMeta)
	if err != nil {
		return err
	}

	for key, val := range cmd.Properties {
		if err := upsertProperty(dialect, sess, now, pathHash, key, val); err != nil {
			return err
		}
	}
	return nil
}

func upsertProperty(dialect migrator.Dialect, sess *sqlstore.DBSession, now time.Time, pathHash string, key string, val string) error {
	existing := &fileMeta{}

	keyEqualsCondition := fmt.Sprintf("%s = ?", dialect.Quote("key"))
	exists, err := sess.Table("file_meta").Where("path_hash = ?", pathHash).Where(keyEqualsCondition, key).Get(existing)
	if err != nil {
		return err
	}

	if exists {
		existing.Value = val
		_, err = sess.Where("path_hash = ?", pathHash).Where(keyEqualsCondition, key).Update(existing)
	} else {
		_, err = sess.Insert(&fileMeta{
			PathHash: pathHash,
			Key:      key,
			Value:    val,
		})
	}
	return err
}

//nolint: gocyclo
func (s dbFileStorage) List(ctx context.Context, folderPath string, paging *Paging, options *ListOptions) (*ListResponse, error) {
	var resp *ListResponse

	err := s.db.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		cursor := ""
		if paging != nil && paging.After != "" {
			pagingFolderPathHash, err := createPathHash(paging.After + Delimiter)
			if err != nil {
				return err
			}

			exists, err := sess.Table("file").Where("path_hash = ?", pagingFolderPathHash).Exist()
			if err != nil {
				return err
			}
			if exists {
				cursor = paging.After + Delimiter
			} else {
				cursor = paging.After
			}
		}

		var foundFiles = make([]*file, 0)
		sess.Table("file")
		lowerFolderPrefix := ""
		lowerFolderPath := strings.ToLower(folderPath)
		if lowerFolderPath == "" || lowerFolderPath == Delimiter {
			lowerFolderPrefix = Delimiter
			lowerFolderPath = Delimiter
		} else {
			lowerFolderPath = strings.TrimSuffix(lowerFolderPath, Delimiter)
			lowerFolderPrefix = lowerFolderPath + Delimiter
		}

		prefixHash, _ := createPathHash(lowerFolderPrefix)

		sess.Where("path_hash != ?", prefixHash)
		parentHash, err := createPathHash(lowerFolderPath)
		if err != nil {
			return err
		}

		if !options.Recursive {
			sess.Where("parent_folder_path_hash = ?", parentHash)
		} else {
			sess.Where("(parent_folder_path_hash = ?) OR (lower(path) LIKE ?)", parentHash, lowerFolderPrefix+"%")
		}

		if !options.WithFolders && options.WithFiles {
			sess.Where("path NOT LIKE ?", "%/")
		}

		if options.WithFolders && !options.WithFiles {
			sess.Where("path LIKE ?", "%/")
		}

		sqlFilter := options.Filter.asSQLFilter()
		sess.Where(sqlFilter.Where, sqlFilter.Args...)

		sess.OrderBy("path")

		pageSize := paging.First
		sess.Limit(pageSize + 1)

		if cursor != "" {
			sess.Where("path > ?", cursor)
		}

		if err := sess.Find(&foundFiles); err != nil {
			return err
		}

		foundLength := len(foundFiles)
		if foundLength > pageSize {
			foundLength = pageSize
		}

		pathToHash := make(map[string]string)
		hashes := make([]string, 0)
		for i := 0; i < foundLength; i++ {
			isFolder := strings.HasSuffix(foundFiles[i].Path, Delimiter)
			if !isFolder {
				hash, err := createPathHash(foundFiles[i].Path)
				if err != nil {
					return err
				}
				hashes = append(hashes, hash)
				pathToHash[foundFiles[i].Path] = hash
			}
		}
		propertiesByPathHash, err := s.getProperties(sess, hashes)
		if err != nil {
			return err
		}

		files := make([]*File, 0)
		for i := 0; i < foundLength; i++ {
			var props map[string]string
			path := strings.TrimSuffix(foundFiles[i].Path, Delimiter)

			if hash, ok := pathToHash[path]; ok {
				if foundProps, ok := propertiesByPathHash[hash]; ok {
					props = foundProps
				} else {
					props = make(map[string]string)
				}
			} else {
				props = make(map[string]string)
			}

			var contents []byte
			if options.WithContents {
				contents = foundFiles[i].Contents
			} else {
				contents = []byte{}
			}
			files = append(files, &File{Contents: contents, FileMetadata: FileMetadata{
				Name:       getName(path),
				FullPath:   path,
				Created:    foundFiles[i].Created,
				Properties: props,
				Modified:   foundFiles[i].Updated,
				Size:       foundFiles[i].Size,
				MimeType:   foundFiles[i].MimeType,
			}})
		}

		lastPath := ""
		if len(files) > 0 {
			lastPath = files[len(files)-1].FullPath
		}

		resp = &ListResponse{
			Files:    files,
			LastPath: lastPath,
			HasMore:  len(foundFiles) == pageSize+1,
		}
		return nil
	})

	return resp, err
}

func (s dbFileStorage) CreateFolder(ctx context.Context, path string) error {
	now := time.Now()
	precedingFolders := precedingFolders(path)

	err := s.db.WithTransactionalDbSession(ctx, func(sess *sqlstore.DBSession) error {
		var insertErr error
		sess.MustLogSQL(true)
		previousFolder := Delimiter
		for i := 0; i < len(precedingFolders); i++ {
			existing := &file{}
			currentFolderParentPath := previousFolder
			previousFolder = Join(previousFolder, getName(precedingFolders[i]))
			currentFolderPath := previousFolder
			if !strings.HasSuffix(currentFolderPath, Delimiter) {
				currentFolderPath = currentFolderPath + Delimiter
			}

			currentFolderPathHash, err := createPathHash(currentFolderPath)
			if err != nil {
				return err
			}

			exists, err := sess.Table("file").Where("path_hash = ?", currentFolderPathHash).Get(existing)
			if err != nil {
				insertErr = err
				break
			}

			if exists {
				previousFolder = strings.TrimSuffix(existing.Path, Delimiter)
				continue
			}

			currentFolderParentPathHash, err := createPathHash(currentFolderParentPath)
			if err != nil {
				return err
			}

			contents := make([]byte, 0)
			file := &file{
				Path:                 currentFolderPath,
				PathHash:             currentFolderPathHash,
				ParentFolderPathHash: currentFolderParentPathHash,
				Contents:             contents,
				ETag:                 createContentsHash(contents),
				Updated:              now,
				MimeType:             DirectoryMimeType,
				Created:              now,
			}
			_, err = sess.Insert(file)
			if err != nil {
				insertErr = err
				break
			}
			s.log.Info("Created folder", "markerPath", file.Path, "parent", currentFolderParentPath)
		}

		if insertErr != nil {
			if rollErr := sess.Rollback(); rollErr != nil {
				return errutil.Wrapf(insertErr, "Rolling back transaction due to error failed: %s", rollErr)
			}
			return insertErr
		}

		return sess.Commit()
	})

	return err
}

func (s dbFileStorage) DeleteFolder(ctx context.Context, folderPath string) error {
	err := s.db.WithDbSession(ctx, func(sess *sqlstore.DBSession) error {
		existing := &file{}
		internalFolderPathHash, err := createPathHash(folderPath + Delimiter)
		if err != nil {
			return err
		}
		exists, err := sess.Table("file").Where("path_hash = ?", internalFolderPathHash).Get(existing)
		if err != nil {
			return err
		}

		if !exists {
			return nil
		}

		_, err = sess.Table("file").Where("path_hash = ?", internalFolderPathHash).Delete(existing)
		return err
	})

	return err
}

func (s dbFileStorage) close() error {
	return nil
}

package finder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/grafana/grafana/pkg/infra/fs"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

var logger = log.New("plugin.finder")

type Finder struct {
	cfg *setting.Cfg
}

func New(cfg *setting.Cfg) Finder {
	return Finder{cfg: cfg}
}

func (f *Finder) Find(pluginDirs []string) ([]string, error) {
	var pluginJSONPaths []string

	for _, dir := range pluginDirs {
		exists, err := fs.Exists(dir)
		if err != nil {
			logger.Warn("Error occurred when checking if plugin directory exists", "dir", dir, "err", err)
		}
		if !exists {
			logger.Warn("Skipping finding plugins as directory does not exist", "dir", dir)
			continue
		}

		paths, err := f.getPluginJSONPaths(dir)
		if err != nil {
			return nil, err
		}
		pluginJSONPaths = append(pluginJSONPaths, paths...)
	}

	return pluginJSONPaths, nil
}

func (f *Finder) getPluginJSONPaths(dir string) ([]string, error) {
	var pluginJSONPaths []string

	var err error
	dir, err = filepath.Abs(dir)
	if err != nil {
		return []string{}, err
	}

	if err := util.Walk(dir, true, true,
		func(currentPath string, fi os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("filepath.Walk reported an error for %q: %w", currentPath, err)
			}

			if fi.Name() == "node_modules" {
				return util.ErrWalkSkipDir
			}

			if fi.IsDir() {
				return nil
			}

			if fi.Name() != "plugin.json" {
				return nil
			}

			pluginJSONPaths = append(pluginJSONPaths, currentPath)
			return nil
		}); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Debug("Couldn't scan directory since it doesn't exist", "pluginDir", dir, "err", err)
			return []string{}, nil
		}
		if errors.Is(err, os.ErrPermission) {
			logger.Debug("Couldn't scan directory due to lack of permissions", "pluginDir", dir, "err", err)
			return []string{}, nil
		}

		return []string{}, err
	}

	return pluginJSONPaths, nil
}

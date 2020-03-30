package setting

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gopkg.in/ini.v1"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	windows = "windows"
)

func TestLoadingSettings(t *testing.T) {

	Convey("Testing loading settings from ini file", t, func() {
		skipStaticRootValidation = true

		Convey("Given the default ini files", func() {
			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{HomePath: "../../"})
			So(err, ShouldBeNil)

			So(AdminUser, ShouldEqual, "admin")
			So(cfg.RendererCallbackUrl, ShouldEqual, "http://localhost:3000/")
		})

		Convey("default.ini should have no semi-colon commented entries", func() {
			file, err := os.Open("../../conf/defaults.ini")
			if err != nil {
				t.Errorf("failed to load defaults.ini file: %v", err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				// This only catches values commented out with ";" and will not catch those that are commented out with "#".
				if strings.HasPrefix(scanner.Text(), ";") {
					t.Errorf("entries in defaults.ini must not be commented or environment variables will not work: %v", scanner.Text())
				}
			}
		})

		Convey("Should be able to override via environment variables", func() {
			os.Setenv("GF_SECURITY_ADMIN_USER", "superduper")

			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{HomePath: "../../"})
			So(err, ShouldBeNil)

			So(AdminUser, ShouldEqual, "superduper")
			So(cfg.DataPath, ShouldEqual, filepath.Join(HomePath, "data"))
			So(cfg.LogsPath, ShouldEqual, filepath.Join(cfg.DataPath, "log"))
		})

		Convey("Should replace password when defined in environment", func() {
			os.Setenv("GF_SECURITY_ADMIN_PASSWORD", "supersecret")

			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{HomePath: "../../"})
			So(err, ShouldBeNil)

			So(appliedEnvOverrides, ShouldContain, "GF_SECURITY_ADMIN_PASSWORD=*********")
		})

		Convey("Should return an error when url is invalid", func() {
			os.Setenv("GF_DATABASE_URL", "postgres.%31://grafana:secret@postgres:5432/grafana")

			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{HomePath: "../../"})

			So(err, ShouldNotBeNil)
		})

		Convey("Should replace password in URL when url environment is defined", func() {
			os.Setenv("GF_DATABASE_URL", "mysql://user:secret@localhost:3306/database")

			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{HomePath: "../../"})
			So(err, ShouldBeNil)

			So(appliedEnvOverrides, ShouldContain, "GF_DATABASE_URL=mysql://user:-redacted-@localhost:3306/database")
		})

		Convey("Should get property map from command line args array", func() {
			props := getCommandLineProperties([]string{"cfg:test=value", "cfg:map.test=1"})

			So(len(props), ShouldEqual, 2)
			So(props["test"], ShouldEqual, "value")
			So(props["map.test"], ShouldEqual, "1")
		})

		Convey("Should be able to override via command line", func() {
			if runtime.GOOS == windows {
				cfg := NewCfg()
				err := cfg.Load(&CommandLineArgs{
					HomePath: "../../",
					Args:     []string{`cfg:paths.data=c:\tmp\data`, `cfg:paths.logs=c:\tmp\logs`},
				})
				So(err, ShouldBeNil)
				So(cfg.DataPath, ShouldEqual, `c:\tmp\data`)
				So(cfg.LogsPath, ShouldEqual, `c:\tmp\logs`)
			} else {
				cfg := NewCfg()
				err := cfg.Load(&CommandLineArgs{
					HomePath: "../../",
					Args:     []string{"cfg:paths.data=/tmp/data", "cfg:paths.logs=/tmp/logs"},
				})
				So(err, ShouldBeNil)

				So(cfg.DataPath, ShouldEqual, "/tmp/data")
				So(cfg.LogsPath, ShouldEqual, "/tmp/logs")
			}
		})

		Convey("Should be able to override defaults via command line", func() {
			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{
				HomePath: "../../",
				Args: []string{
					"cfg:default.server.domain=test2",
				},
				Config: filepath.Join(HomePath, "pkg/setting/testdata/override.ini"),
			})
			So(err, ShouldBeNil)

			So(Domain, ShouldEqual, "test2")
		})

		Convey("Defaults can be overridden in specified config file", func() {
			if runtime.GOOS == windows {
				cfg := NewCfg()
				err := cfg.Load(&CommandLineArgs{
					HomePath: "../../",
					Config:   filepath.Join(HomePath, "pkg/setting/testdata/override_windows.ini"),
					Args:     []string{`cfg:default.paths.data=c:\tmp\data`},
				})
				So(err, ShouldBeNil)

				So(cfg.DataPath, ShouldEqual, `c:\tmp\override`)
			} else {
				cfg := NewCfg()
				err := cfg.Load(&CommandLineArgs{
					HomePath: "../../",
					Config:   filepath.Join(HomePath, "pkg/setting/testdata/override.ini"),
					Args:     []string{"cfg:default.paths.data=/tmp/data"},
				})
				So(err, ShouldBeNil)

				So(cfg.DataPath, ShouldEqual, "/tmp/override")
			}
		})

		Convey("Command line overrides specified config file", func() {
			if runtime.GOOS == windows {
				cfg := NewCfg()
				err := cfg.Load(&CommandLineArgs{
					HomePath: "../../",
					Config:   filepath.Join(HomePath, "pkg/setting/testdata/override_windows.ini"),
					Args:     []string{`cfg:paths.data=c:\tmp\data`},
				})
				So(err, ShouldBeNil)

				So(cfg.DataPath, ShouldEqual, `c:\tmp\data`)
			} else {
				cfg := NewCfg()
				err := cfg.Load(&CommandLineArgs{
					HomePath: "../../",
					Config:   filepath.Join(HomePath, "pkg/setting/testdata/override.ini"),
					Args:     []string{"cfg:paths.data=/tmp/data"},
				})
				So(err, ShouldBeNil)

				So(cfg.DataPath, ShouldEqual, "/tmp/data")
			}
		})

		Convey("Can use environment variables in config values", func() {
			if runtime.GOOS == windows {
				os.Setenv("GF_DATA_PATH", `c:\tmp\env_override`)
				cfg := NewCfg()
				err := cfg.Load(&CommandLineArgs{
					HomePath: "../../",
					Args:     []string{"cfg:paths.data=${GF_DATA_PATH}"},
				})
				So(err, ShouldBeNil)

				So(cfg.DataPath, ShouldEqual, `c:\tmp\env_override`)
			} else {
				os.Setenv("GF_DATA_PATH", "/tmp/env_override")
				cfg := NewCfg()
				err := cfg.Load(&CommandLineArgs{
					HomePath: "../../",
					Args:     []string{"cfg:paths.data=${GF_DATA_PATH}"},
				})
				So(err, ShouldBeNil)

				So(cfg.DataPath, ShouldEqual, "/tmp/env_override")
			}
		})

		Convey("instance_name default to hostname even if hostname env is empty", func() {
			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{
				HomePath: "../../",
			})
			So(err, ShouldBeNil)

			hostname, _ := os.Hostname()
			So(InstanceName, ShouldEqual, hostname)
		})

		Convey("Reading callback_url should add trailing slash", func() {
			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{
				HomePath: "../../",
				Args:     []string{"cfg:rendering.callback_url=http://myserver/renderer"},
			})
			So(err, ShouldBeNil)

			So(cfg.RendererCallbackUrl, ShouldEqual, "http://myserver/renderer/")
		})

		Convey("Only sync_ttl should return the value sync_ttl", func() {
			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{
				HomePath: "../../",
				Args:     []string{"cfg:auth.proxy.sync_ttl=2"},
			})
			So(err, ShouldBeNil)

			So(AuthProxySyncTtl, ShouldEqual, 2)
		})

		Convey("Only ldap_sync_ttl should return the value ldap_sync_ttl", func() {
			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{
				HomePath: "../../",
				Args:     []string{"cfg:auth.proxy.ldap_sync_ttl=5"},
			})
			So(err, ShouldBeNil)

			So(AuthProxySyncTtl, ShouldEqual, 5)
		})

		Convey("ldap_sync should override ldap_sync_ttl that is default value", func() {
			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{
				HomePath: "../../",
				Args:     []string{"cfg:auth.proxy.sync_ttl=5"},
			})
			So(err, ShouldBeNil)

			So(AuthProxySyncTtl, ShouldEqual, 5)
		})

		Convey("ldap_sync should not override ldap_sync_ttl that is different from default value", func() {
			cfg := NewCfg()
			err := cfg.Load(&CommandLineArgs{
				HomePath: "../../",
				Args:     []string{"cfg:auth.proxy.ldap_sync_ttl=12", "cfg:auth.proxy.sync_ttl=5"},
			})
			So(err, ShouldBeNil)

			So(AuthProxySyncTtl, ShouldEqual, 12)
		})
	})

	Convey("Test reading string values from .ini file", t, func() {

		iniFile, err := ini.Load(path.Join(HomePath, "pkg/setting/testdata/invalid.ini"))
		So(err, ShouldBeNil)

		Convey("If key is found - should return value from ini file", func() {
			value, err := valueAsString(iniFile.Section("server"), "alt_url", "")
			So(err, ShouldBeNil)
			So(value, ShouldEqual, "https://grafana.com/")
		})

		Convey("If key is not found - should return default value", func() {
			value, err := valueAsString(iniFile.Section("server"), "extra_url", "default_url_val")
			So(err, ShouldBeNil)
			So(value, ShouldEqual, "default_url_val")
		})

		Convey("In case of panic - should return user-friendly error", func() {
			value, err := valueAsString(iniFile.Section("server"), "root_url", "")
			So(err.Error(), ShouldEqual, "Invalid value for key 'root_url' in configuration file")
			So(value, ShouldEqual, "")
		})

	})
}

func TestParseAppUrlAndSubUrl(t *testing.T) {
	testCases := []struct {
		rootURL           string
		expectedAppURL    string
		expectedAppSubURL string
	}{
		{rootURL: "http://localhost:3000/", expectedAppURL: "http://localhost:3000/"},
		{rootURL: "http://localhost:3000", expectedAppURL: "http://localhost:3000/"},
		{rootURL: "http://localhost:3000/grafana", expectedAppURL: "http://localhost:3000/grafana/", expectedAppSubURL: "/grafana"},
		{rootURL: "http://localhost:3000/grafana/", expectedAppURL: "http://localhost:3000/grafana/", expectedAppSubURL: "/grafana"},
	}

	for _, tc := range testCases {
		f := ini.Empty()
		s, err := f.NewSection("server")
		require.NoError(t, err)
		_, err = s.NewKey("root_url", tc.rootURL)
		require.NoError(t, err)
		appURL, appSubURL, err := parseAppUrlAndSubUrl(s)
		require.NoError(t, err)
		require.Equal(t, tc.expectedAppURL, appURL)
		require.Equal(t, tc.expectedAppSubURL, appSubURL)
	}
}

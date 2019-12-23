package rendering

import (
	"context"
	"fmt"
	plugin "github.com/hashicorp/go-plugin"
	"net/url"
	"os"
	"path/filepath"

	pluginModel "github.com/grafana/grafana-plugin-model/go/renderer"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/middleware"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/registry"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
)

func init() {
	registry.RegisterService(&RenderingService{})
}

type RenderingService struct {
	log             log.Logger
	pluginClient    *plugin.Client
	grpcPlugin      pluginModel.RendererPlugin
	pluginInfo      *plugins.RendererPlugin
	renderAction    renderFunc
	domain          string
	inProgressCount int

	Cfg *setting.Cfg `inject:""`
}

func (rs *RenderingService) Init() error {
	rs.log = log.New("rendering")

	// ensure ImagesDir exists
	err := os.MkdirAll(rs.Cfg.ImagesDir, 0700)
	if err != nil {
		return err
	}

	// set value used for domain attribute of renderKey cookie
	if rs.Cfg.RendererUrl != "" {
		// RendererCallbackUrl has already been passed, it won't generate an error.
		u, _ := url.Parse(rs.Cfg.RendererCallbackUrl)
		rs.domain = u.Hostname()
	} else if setting.HttpAddr != setting.DEFAULT_HTTP_ADDR {
		rs.domain = setting.HttpAddr
	} else {
		rs.domain = "localhost"
	}

	return nil
}

func (rs *RenderingService) Run(ctx context.Context) error {
	if rs.Cfg.RendererUrl != "" {
		rs.log.Info("Backend rendering via external http server")
		rs.renderAction = rs.renderViaHttp
		<-ctx.Done()
		return nil
	}

	if plugins.Renderer == nil {
		rs.log.Info("Backend rendering via phantomJS")
		rs.log.Warn("phantomJS is deprecated and will be removed in a future release. " +
			"You should consider migrating from phantomJS to grafana-image-renderer plugin.")
		rs.renderAction = rs.renderViaPhantomJS
		<-ctx.Done()
		return nil
	}

	rs.pluginInfo = plugins.Renderer

	if err := rs.startPlugin(ctx); err != nil {
		return err
	}

	rs.renderAction = rs.renderViaPlugin

	err := rs.watchAndRestartPlugin(ctx)

	if rs.pluginClient != nil {
		rs.log.Debug("Killing renderer plugin process")
		rs.pluginClient.Kill()
	}

	return err
}

func (rs *RenderingService) RenderErrorImage(err error) (*RenderResult, error) {
	imgUrl := "public/img/rendering_error.png"

	return &RenderResult{
		FilePath: filepath.Join(setting.HomePath, imgUrl),
	}, nil
}

func (rs *RenderingService) Render(ctx context.Context, opts Opts) (*RenderResult, error) {
	if rs.inProgressCount > opts.ConcurrentLimit {
		return &RenderResult{
			FilePath: filepath.Join(setting.HomePath, "public/img/rendering_limit.png"),
		}, nil
	}

	defer func() {
		rs.inProgressCount -= 1
	}()

	rs.inProgressCount += 1

	if rs.renderAction != nil {
		return rs.renderAction(ctx, opts)
	}
	return nil, fmt.Errorf("No renderer found")
}

func (rs *RenderingService) getFilePathForNewImage() (string, error) {
	rand, err := util.GetRandomString(20)
	if err != nil {
		return "", err
	}
	pngPath, err := filepath.Abs(filepath.Join(rs.Cfg.ImagesDir, rand))
	if err != nil {
		return "", err
	}

	return pngPath + ".png", nil
}

func (rs *RenderingService) getURL(path string) string {
	if rs.Cfg.RendererUrl != "" {
		// The backend rendering service can potentially be remote.
		// So we need to use the root_url to ensure the rendering service
		// can reach this Grafana instance.

		// &render=1 signals to the legacy redirect layer to
		return fmt.Sprintf("%s%s&render=1", rs.Cfg.RendererCallbackUrl, path)

	}
	// &render=1 signals to the legacy redirect layer to
	return fmt.Sprintf("%s://%s:%s/%s&render=1", setting.Protocol, rs.domain, setting.HttpPort, path)
}

func (rs *RenderingService) getRenderKey(orgId, userId int64, orgRole models.RoleType) (string, error) {
	return middleware.AddRenderAuthKey(orgId, userId, orgRole)
}

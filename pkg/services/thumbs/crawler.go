package thumbs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/live"
	"github.com/grafana/grafana/pkg/services/rendering"
)

type simpleCrawler struct {
	renderService rendering.Service
	threadCount   int

	glive                   *live.GrafanaLive
	thumbnailRepo           thumbnailRepo
	mode                    CrawlerMode
	thumbnailKind           models.ThumbnailKind
	opts                    rendering.Opts
	status                  crawlStatus
	statusMutex             sync.RWMutex
	queue                   []*models.DashboardWithStaleThumbnail
	queueMutex              sync.Mutex
	log                     log.Logger
	renderingSessionByOrgId map[int64]rendering.Session
}

func newSimpleCrawler(renderService rendering.Service, gl *live.GrafanaLive, repo thumbnailRepo) dashRenderer {
	c := &simpleCrawler{
		renderService: renderService,
		threadCount:   6,
		glive:         gl,
		thumbnailRepo: repo,
		log:           log.New("thumbnails_crawler"),
		status: crawlStatus{
			State:    initializing,
			Complete: 0,
			Queue:    0,
		},
		renderingSessionByOrgId: make(map[int64]rendering.Session),
		queue:                   nil,
	}
	c.broadcastStatus()
	return c
}

func (r *simpleCrawler) next(ctx context.Context) (*models.DashboardWithStaleThumbnail, rendering.Session, rendering.AuthOpts, error) {
	r.queueMutex.Lock()
	defer r.queueMutex.Unlock()

	if r.queue == nil || len(r.queue) < 1 {
		return nil, nil, rendering.AuthOpts{}, nil
	}

	v := r.queue[0]
	r.queue = r.queue[1:]

	authOpts := rendering.AuthOpts{
		OrgID:   v.OrgId,
		UserID:  r.opts.AuthOpts.UserID,
		OrgRole: r.opts.AuthOpts.OrgRole,
	}

	if renderingSession, ok := r.renderingSessionByOrgId[v.OrgId]; ok {
		return v, renderingSession, authOpts, nil
	}

	renderingSession, err := r.renderService.CreateRenderingSession(ctx, authOpts, rendering.SessionOpts{
		Expiry:                     5 * time.Minute,
		RefreshExpiryOnEachRequest: true,
	})

	if err != nil {
		return nil, nil, authOpts, err
	}

	r.renderingSessionByOrgId[v.OrgId] = renderingSession
	return v, renderingSession, authOpts, nil
}

func (r *simpleCrawler) broadcastStatus() {
	s, err := r.Status()
	if err != nil {
		r.log.Warn("Error reading status", "err", err)
		return
	}
	msg, err := json.Marshal(s)
	if err != nil {
		r.log.Warn("Error making message", "err", err)
		return
	}
	err = r.glive.Publish(r.opts.OrgID, "grafana/broadcast/crawler", msg)
	if err != nil {
		r.log.Warn("Error Publish message", "err", err)
		return
	}
}

type byOrgId []*models.DashboardWithStaleThumbnail

func (d byOrgId) Len() int           { return len(d) }
func (d byOrgId) Less(i, j int) bool { return d[i].OrgId > d[j].OrgId }
func (d byOrgId) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }

func (r *simpleCrawler) Run(ctx context.Context, authOpts rendering.AuthOpts, mode CrawlerMode, theme models.Theme, thumbnailKind models.ThumbnailKind) error {
	res, err := r.renderService.HasCapability(rendering.ScalingDownImages)
	if err != nil {
		return err
	}

	if !res.IsSupported {
		return fmt.Errorf("cant run dashboard crawler - rendering service needs to be updated. "+
			"current version: %s, requiredVersion: %s", r.renderService.Version(), res.SemverConstraint)
	}

	r.queueMutex.Lock()
	if r.IsRunning() {
		r.queueMutex.Unlock()
		r.log.Info("Already running")
		return nil
	}

	now := time.Now()

	items, err := r.thumbnailRepo.findDashboardsWithStaleThumbnails(ctx, theme, thumbnailKind)
	if err != nil {
		r.log.Error("Error when fetching dashboards with stale thumbnails", "err", err.Error())
		r.queueMutex.Unlock()
		return err
	}

	if len(items) == 0 {
		r.queueMutex.Unlock()
		return nil
	}

	// sort the items so that we render all items from each org before moving on to the next one
	// helps us avoid having to maintain multiple active rendering sessions
	sort.Sort(byOrgId(items))

	r.mode = mode
	r.thumbnailKind = thumbnailKind
	r.opts = rendering.Opts{
		AuthOpts: authOpts,
		TimeoutOpts: rendering.TimeoutOpts{
			Timeout:                  20 * time.Second,
			RequestTimeoutMultiplier: 3,
		},
		Theme:           theme,
		ConcurrentLimit: 10,
	}

	r.renderingSessionByOrgId = make(map[int64]rendering.Session)
	r.queue = items
	r.status = crawlStatus{
		Started:  now,
		State:    running,
		Complete: 0,
	}
	r.broadcastStatus()
	r.queueMutex.Unlock()

	r.log.Info("Starting dashboard crawler", "dashboardsToCrawl", len(items), "mode", string(mode), "theme", string(theme), "kind", string(thumbnailKind))

	group, gCtx := errgroup.WithContext(ctx)
	// create a pool of workers
	for i := 0; i < r.threadCount; i++ {
		walkerId := i
		group.Go(func() error {
			r.walk(gCtx, walkerId)
			return nil
		})
	}

	err = group.Wait()
	if err != nil {
		r.log.Error("Crawl ended with an error", "err", err)
	}

	r.crawlFinished()
	r.broadcastStatus()
	return err
}

func (r *simpleCrawler) IsRunning() bool {
	r.statusMutex.Lock()
	defer r.statusMutex.Unlock()
	return r.status.State == running
}

func (r *simpleCrawler) Stop() (crawlStatus, error) {
	r.statusMutex.Lock()
	if r.status.State == running {
		r.status.State = stopping
	}
	r.statusMutex.Unlock()

	return r.Status()
}

func (r *simpleCrawler) Status() (crawlStatus, error) {
	r.statusMutex.RLock()
	defer r.statusMutex.RUnlock()

	status := crawlStatus{
		State:    r.status.State,
		Started:  r.status.Started,
		Complete: r.status.Complete,
		Errors:   r.status.Errors,
		Queue:    len(r.queue),
		Last:     r.status.Last,
	}
	return status, nil
}

func (r *simpleCrawler) newErrorResult() {
	r.statusMutex.Lock()
	defer r.statusMutex.Unlock()

	r.status.Errors++
	r.status.Last = time.Now()
}

func (r *simpleCrawler) newSuccessResult() {
	r.statusMutex.Lock()
	defer r.statusMutex.Unlock()

	r.status.Complete++
	r.status.Last = time.Now()
}

func (r *simpleCrawler) crawlFinished() {
	r.statusMutex.Lock()
	defer r.statusMutex.Unlock()

	r.status.State = stopped
	r.status.Finished = time.Now()
	r.log.Info("Crawler finished", "startTime", r.status.Started, "endTime", r.status.Finished, "durationInSeconds", int64(time.Since(r.status.Started)/time.Second))
}

func (r *simpleCrawler) shouldWalk() bool {
	r.statusMutex.RLock()
	defer r.statusMutex.RUnlock()

	return r.status.State == running
}

func (r *simpleCrawler) walk(ctx context.Context, id int) {
	for {
		if !r.shouldWalk() {
			break
		}

		item, renderingSession, authOpts, err := r.next(ctx)
		if err != nil {
			r.log.Error("Render item retrieval error", "walkerId", id, "error", err)
			break
		}

		if item == nil || renderingSession == nil {
			break
		}

		url := models.GetKioskModeDashboardUrl(item.Uid, item.Slug, r.opts.Theme)
		r.log.Info("Getting dashboard thumbnail", "walkerId", id, "dashboardUID", item.Uid, "url", url)

		res, err := r.renderService.Render(ctx, rendering.Opts{
			Width:             320,
			Height:            240,
			Path:              strings.TrimPrefix(url, "/"),
			AuthOpts:          authOpts,
			TimeoutOpts:       r.opts.TimeoutOpts,
			ConcurrentLimit:   r.opts.ConcurrentLimit,
			Theme:             r.opts.Theme,
			DeviceScaleFactor: -5, // negative numbers will render larger and then scale down.
		}, renderingSession)
		if err != nil {
			r.log.Warn("Error getting image", "walkerId", id, "dashboardUID", item.Uid, "url", url, "err", err)
			r.newErrorResult()
		} else if res.FilePath == "" {
			r.log.Warn("Error getting image... no response", "walkerId", id, "dashboardUID", item.Uid, "url", url)
			r.newErrorResult()
		} else if strings.Contains(res.FilePath, "public/img") {
			r.log.Warn("Error getting image... internal result", "walkerId", id, "dashboardUID", item.Uid, "url", url, "img", res.FilePath)
			// rendering service returned a static error image - we should not remove that file
			r.newErrorResult()
		} else {
			func() {
				defer func() {
					err := os.Remove(res.FilePath)
					if err != nil {
						r.log.Error("Failed to remove thumbnail temp file", "walkerId", id, "dashboardUID", item.Uid, "url", url, "err", err)
					}
				}()

				thumbnailId, err := r.thumbnailRepo.saveFromFile(ctx, res.FilePath, models.DashboardThumbnailMeta{
					DashboardUID: item.Uid,
					OrgId:        item.OrgId,
					Theme:        r.opts.Theme,
					Kind:         r.thumbnailKind,
				}, item.Version)

				if err != nil {
					r.log.Warn("Error saving image image", "walkerId", id, "dashboardUID", item.Uid, "url", url, "err", err)
					r.newErrorResult()
				} else {
					r.log.Info("Saved thumbnail", "walkerId", id, "dashboardUID", item.Uid, "url", url, "thumbnailId", thumbnailId)
					r.newSuccessResult()
				}
			}()
		}
		r.broadcastStatus()
	}

	r.log.Info("Walker finished", "walkerId", id)
}

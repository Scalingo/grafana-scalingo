package export

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/grafana/grafana/pkg/infra/log"
)

var _ Job = new(dummyExportJob)

type dummyExportJob struct {
	logger log.Logger

	statusMu    sync.Mutex
	status      ExportStatus
	cfg         ExportConfig
	broadcaster statusBroadcaster
}

func startDummyExportJob(cfg ExportConfig, broadcaster statusBroadcaster) (Job, error) {
	if cfg.Format != "git" {
		return nil, errors.New("only git format is supported")
	}

	job := &dummyExportJob{
		logger:      log.New("dummy_export_job"),
		cfg:         cfg,
		broadcaster: broadcaster,
		status: ExportStatus{
			Running: true,
			Target:  "git export",
			Started: time.Now().UnixMilli(),
			Count:   int64(math.Round(10 + rand.Float64()*20)),
			Current: 0,
		},
	}

	broadcaster(job.status)
	go job.start()
	return job, nil
}

func (e *dummyExportJob) start() {
	defer func() {
		e.logger.Info("Finished dummy export job")

		e.statusMu.Lock()
		defer e.statusMu.Unlock()
		s := e.status
		if err := recover(); err != nil {
			e.logger.Error("export panic", "error", err)
			s.Status = fmt.Sprintf("ERROR: %v", err)
		}
		// Make sure it finishes OK
		if s.Finished < 10 {
			s.Finished = time.Now().UnixMilli()
		}
		s.Running = false
		if s.Status == "" {
			s.Status = "done"
		}
		e.status = s
		e.broadcaster(s)
	}()

	e.logger.Info("Starting dummy export job")

	ticker := time.NewTicker(1 * time.Second)
	for t := range ticker.C {
		e.statusMu.Lock()
		e.status.Changed = t.UnixMilli()
		e.status.Current++
		e.status.Last = fmt.Sprintf("ITEM: %d", e.status.Current)
		e.statusMu.Unlock()

		// Wait till we are done
		shouldStop := e.status.Current >= e.status.Count
		e.broadcaster(e.status)

		if shouldStop {
			break
		}
	}
}

func (e *dummyExportJob) getStatus() ExportStatus {
	e.statusMu.Lock()
	defer e.statusMu.Unlock()

	return e.status
}

func (e *dummyExportJob) getConfig() ExportConfig {
	e.statusMu.Lock()
	defer e.statusMu.Unlock()

	return e.cfg
}

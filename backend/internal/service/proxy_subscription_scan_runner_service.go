package service

import (
	"context"
	"database/sql"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
	"github.com/shirou/gopsutil/v4/mem"
)

const proxySubscriptionScanRunnerSchedule = "* * * * *"

type ProxySubscriptionScanRunnerService struct {
	adminSvc  AdminService
	entClient *dbent.Client
	cfg       *config.Config

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once
	runMu     sync.Mutex
	running   bool
}

type proxySubscriptionScanRunnerSource struct {
	ID                  int64
	Name                string
	ScanIntervalMinutes int
	LastScanAt          sql.NullTime
	Strategy            ProxySubscriptionStrategy
}

func NewProxySubscriptionScanRunnerService(adminSvc AdminService, entClient *dbent.Client, cfg *config.Config) *ProxySubscriptionScanRunnerService {
	return &ProxySubscriptionScanRunnerService{
		adminSvc:  adminSvc,
		entClient: entClient,
		cfg:       cfg,
	}
}

func (s *ProxySubscriptionScanRunnerService) Start() {
	if s == nil || s.adminSvc == nil || s.entClient == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}
		c := cron.New(cron.WithLocation(loc))
		_, err := c.AddFunc(proxySubscriptionScanRunnerSchedule, func() { s.runDueScan() })
		if err != nil {
			logger.LegacyPrintf("service.proxy_subscription_scan_runner", "[ProxySubscriptionScanRunner] not started: %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.proxy_subscription_scan_runner", "[ProxySubscriptionScanRunner] started (tick=every minute)")
	})
}

func (s *ProxySubscriptionScanRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.proxy_subscription_scan_runner", "[ProxySubscriptionScanRunner] cron stop timed out")
			}
		}
	})
}

func (s *ProxySubscriptionScanRunnerService) runDueScan() {
	if !s.tryStartRun() {
		logger.LegacyPrintf("service.proxy_subscription_scan_runner", "[ProxySubscriptionScanRunner] tick skipped: scan already running")
		return
	}
	defer s.finishRun()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Minute)
	defer cancel()

	source, err := s.findNextDueSource(ctx)
	if err != nil {
		logger.LegacyPrintf("service.proxy_subscription_scan_runner", "[ProxySubscriptionScanRunner] find due source failed: %v", err)
		return
	}
	if source == nil {
		return
	}
	if blocked, freeMB := s.shouldDelaySource(source.Strategy); blocked {
		logger.LegacyPrintf(
			"service.proxy_subscription_scan_runner",
			"[ProxySubscriptionScanRunner] delayed source=%d name=%q free_memory_mb=%d",
			source.ID,
			source.Name,
			freeMB,
		)
		return
	}

	scanTimeout := time.Duration(source.Strategy.ScanBudgetMaxMinutes) * time.Minute
	if scanTimeout <= 0 {
		scanTimeout = 40 * time.Minute
	}
	scanCtx, scanCancel := context.WithTimeout(ctx, scanTimeout)
	defer scanCancel()

	logger.LegacyPrintf(
		"service.proxy_subscription_scan_runner",
		"[ProxySubscriptionScanRunner] scanning source=%d name=%q interval=%dm",
		source.ID,
		source.Name,
		source.ScanIntervalMinutes,
	)
	result, err := s.adminSvc.ScanProxySubscriptionSource(scanCtx, source.ID)
	if err != nil {
		logger.LegacyPrintf("service.proxy_subscription_scan_runner", "[ProxySubscriptionScanRunner] source=%d failed: %v", source.ID, err)
		return
	}
	logger.LegacyPrintf(
		"service.proxy_subscription_scan_runner",
		"[ProxySubscriptionScanRunner] source=%d done parsed=%d selected=%d saved=%d errors=%d",
		source.ID,
		result.Parsed,
		result.Selected,
		result.Saved,
		len(result.Errors),
	)
}

func (s *ProxySubscriptionScanRunnerService) tryStartRun() bool {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *ProxySubscriptionScanRunnerService) finishRun() {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.running = false
}

func (s *ProxySubscriptionScanRunnerService) findNextDueSource(ctx context.Context) (*proxySubscriptionScanRunnerSource, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT id, name, scan_interval_minutes, last_scan_at, COALESCE(strategy_json::text, '{}')
FROM proxy_subscription_sources
WHERE deleted_at IS NULL
  AND status = 'active'
  AND scan_enabled = TRUE
  AND (
    last_scan_at IS NULL
    OR last_scan_at <= NOW() - ((GREATEST(scan_interval_minutes, 5))::text || ' minutes')::interval
  )
ORDER BY COALESCE(last_scan_at, TO_TIMESTAMP(0)) ASC, id ASC
LIMIT 1`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var source proxySubscriptionScanRunnerSource
	var strategyRaw string
	if err := rows.Scan(&source.ID, &source.Name, &source.ScanIntervalMinutes, &source.LastScanAt, &strategyRaw); err != nil {
		return nil, err
	}
	source.Strategy = parseProxySubscriptionStrategy(strategyRaw)
	return &source, rows.Err()
}

func (s *ProxySubscriptionScanRunnerService) shouldDelaySource(strategy ProxySubscriptionStrategy) (bool, int64) {
	strategy = normalizeProxySubscriptionStrategy(strategy)
	if !strategy.ResourceAdaptiveScan {
		return false, 0
	}
	vm, err := mem.VirtualMemory()
	if err != nil || vm == nil {
		return false, 0
	}
	freeMB := int64(vm.Available / 1024 / 1024)
	if strategy.PauseFreeMemoryMB > 0 && freeMB < int64(strategy.PauseFreeMemoryMB) {
		return true, freeMB
	}
	return false, freeMB
}

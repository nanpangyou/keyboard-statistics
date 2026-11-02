// Package service 把采集、存储、聚合串起来，是 UI 唯一依赖的门面。
package service

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"keyboard-statistics/internal/collect"
	"keyboard-statistics/internal/hook"
	"keyboard-statistics/internal/stats"
	"keyboard-statistics/internal/store"
)

const (
	// flushInterval 是后台落库间隔：太频繁浪费 IO，太稀疏断电会丢数据
	flushInterval = 30 * time.Second
	// flushThreshold 是提前落库的桶数阈值，打字快时不必等满 30 秒
	flushThreshold = 500

	metaCollecting = "collecting"
	metaOnboarded  = "onboarded"
)

// Options 是构造 Service 的依赖。
type Options struct {
	DBPath string
	Now    func() time.Time
	// StartHook 为 false 时不装键盘钩子，供单元测试使用
	StartHook bool
}

// Snapshot 是某个时间范围下界面需要的全部数据。
type Snapshot struct {
	From          string
	To            string
	Days          int
	TotalKeys     int64
	ActiveSeconds int64
	DailyAverage  int64
	PeakDay       string
	PeakCount     int64
	TopKeys       []stats.KeyCount
	TopCombos     []stats.ComboCount
	Hourly        [24]int64
	KeyCounts     map[uint16]int64
	ComboCounts   map[string]int64
}

// Service 是应用门面：实现 hook.Handler，同时向 UI 提供查询。
type Service struct {
	db  *store.Store
	col *collect.Collector
	now func() time.Time
	opt Options

	collecting atomic.Bool

	hookMu sync.Mutex
	hook   *hook.Hook

	loopDone  chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// New 打开数据库并准备采集器，此时还没开始采集。
func New(opts Options) (*Service, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	db, err := store.Open(opts.DBPath)
	if err != nil {
		return nil, fmt.Errorf("打开统计库: %w", err)
	}

	s := &Service{
		db:       db,
		col:      collect.New(opts.Now),
		now:      opts.Now,
		opt:      opts,
		done:     make(chan struct{}),
		loopDone: make(chan struct{}),
	}

	// 默认开始采集：用户装了它就是想要统计，不该让他再点一次开关
	s.collecting.Store(true)
	if v, ok, err := db.Meta(metaCollecting); err == nil && ok && v == "0" {
		s.collecting.Store(false)
	}

	// 落库循环在 New 里就开起来，这样 Close 永远有一个真实存在的 goroutine 可等，
	// 不会出现「没调 Start 就 Close」时的死锁。
	go s.loop()
	return s, nil
}

// Start 安装键盘钩子。落库循环已在 New 中启动。
func (s *Service) Start() error {
	if s.opt.StartHook {
		h, err := hook.Start(s)
		if err != nil {
			return fmt.Errorf("启动键盘钩子: %w", err)
		}
		s.hookMu.Lock()
		s.hook = h
		s.hookMu.Unlock()
	}
	return nil
}

func (s *Service) loop() {
	defer close(s.loopDone)

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			if s.col.Pending() >= flushThreshold {
				_ = s.FlushNow()
				continue
			}
			_ = s.FlushNow()
		}
	}
}

// Close 停止采集、落库并关闭数据库。重复调用安全。
func (s *Service) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		<-s.loopDone

		s.hookMu.Lock()
		h := s.hook
		s.hook = nil
		s.hookMu.Unlock()
		if h != nil {
			if err := h.Stop(); err != nil {
				s.closeErr = fmt.Errorf("停止键盘钩子: %w", err)
			}
		}

		if err := s.FlushNow(); err != nil && s.closeErr == nil {
			s.closeErr = err
		}
		if err := s.db.Close(); err != nil && s.closeErr == nil {
			s.closeErr = fmt.Errorf("关闭统计库: %w", err)
		}
	})
	return s.closeErr
}

// KeyDown 实现 hook.Handler。
func (s *Service) KeyDown(vk uint16) {
	if !s.collecting.Load() {
		return
	}
	s.col.KeyDown(vk)
}

// KeyUp 实现 hook.Handler。
func (s *Service) KeyUp(vk uint16) {
	s.col.KeyUp(vk)
}

// Collecting 报告当前是否在采集。
func (s *Service) Collecting() bool { return s.collecting.Load() }

// SetCollecting 切换采集开关，并把状态持久化给下次启动。
func (s *Service) SetCollecting(on bool) {
	s.collecting.Store(on)
	if on {
		s.col.Reset() // 从暂停恢复时丢掉旧的按键按住状态
	}
	_ = s.db.SetMeta(metaCollecting, boolToMeta(on))
}

// FlushNow 立即把内存缓冲写入数据库。
func (s *Service) FlushNow() error {
	b := s.col.Flush()
	if len(b.Keys) == 0 && len(b.Combos) == 0 && len(b.Activity) == 0 {
		return nil
	}
	if err := s.db.AddKeyCounts(b.Keys); err != nil {
		return fmt.Errorf("写入按键: %w", err)
	}
	if err := s.db.AddComboCounts(b.Combos); err != nil {
		return fmt.Errorf("写入组合键: %w", err)
	}
	for _, a := range b.Activity {
		if err := s.db.AddActivity(a.Day, a.Sec, a.FirstPress, a.LastPress); err != nil {
			return fmt.Errorf("写入活跃时长: %w", err)
		}
	}
	return nil
}

// Snapshot 聚合指定时间范围的数据。
func (s *Service) Snapshot(scope stats.Scope) (Snapshot, error) {
	firstDay, err := s.db.FirstDay()
	if err != nil {
		return Snapshot{}, err
	}
	from, to := stats.Range(s.now(), scope, firstDay)

	keys, err := s.db.KeyCounts(from, to)
	if err != nil {
		return Snapshot{}, err
	}
	combos, err := s.db.ComboCounts(from, to)
	if err != nil {
		return Snapshot{}, err
	}
	hours, err := s.db.HourCounts(from, to)
	if err != nil {
		return Snapshot{}, err
	}
	daily, err := s.db.DailyTotals(from, to)
	if err != nil {
		return Snapshot{}, err
	}
	active, err := s.db.ActiveSeconds(from, to)
	if err != nil {
		return Snapshot{}, err
	}

	var total int64
	for _, c := range keys {
		total += c
	}
	peakDay, peakCount, _ := stats.Peak(daily)
	days := stats.DaysInclusive(from, to)

	return Snapshot{
		From:          from,
		To:            to,
		Days:          days,
		TotalKeys:     total,
		ActiveSeconds: active,
		DailyAverage:  stats.DailyAverage(total, days),
		PeakDay:       peakDay,
		PeakCount:     peakCount,
		TopKeys:       stats.TopKeys(keys, 10),
		TopCombos:     stats.TopCombos(combos, 10),
		Hourly:        stats.HourlyDistribution(hours),
		KeyCounts:     keys,
		ComboCounts:   combos,
	}, nil
}

// ClearData 清空全部统计数据，但保留采集开关与引导标记。
func (s *Service) ClearData() error {
	if err := s.db.ClearAll(); err != nil {
		return fmt.Errorf("清空统计数据: %w", err)
	}
	return nil
}

// Onboarded 报告是否已经展示过首次启动说明。
func (s *Service) Onboarded() (bool, error) {
	v, ok, err := s.db.Meta(metaOnboarded)
	if err != nil {
		return false, err
	}
	return ok && v == "1", nil
}

// MarkOnboarded 记录首次启动说明已展示。
func (s *Service) MarkOnboarded() error {
	return s.db.SetMeta(metaOnboarded, "1")
}

func boolToMeta(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

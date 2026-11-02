// Package store 负责 SQLite 持久化：建表、批量累加写入、按时间范围查询。
//
// 采集线程只往内存里累加，由后台按 30 秒或 500 次按键 flush 一次，
// 所以这里的写法都以「批量 upsert 累加」为核心，不做逐键写入。
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // 纯 Go 的 sqlite 驱动，不引入 cgo

	"keyboard-statistics/internal/stats"
)

// SchemaVersion 是当前表结构的版本号，写进 meta 表用于将来迁移。
const SchemaVersion = "1"

const schemaSQL = `
CREATE TABLE IF NOT EXISTS key_count (
    day   TEXT    NOT NULL,
    hour  INTEGER NOT NULL,
    vk    INTEGER NOT NULL,
    count INTEGER NOT NULL,
    PRIMARY KEY (day, hour, vk)
);

CREATE TABLE IF NOT EXISTS combo_count (
    day   TEXT    NOT NULL,
    hour  INTEGER NOT NULL,
    combo TEXT    NOT NULL,
    count INTEGER NOT NULL,
    PRIMARY KEY (day, hour, combo)
);

CREATE TABLE IF NOT EXISTS day_activity (
    day         TEXT PRIMARY KEY,
    active_sec  INTEGER NOT NULL DEFAULT 0,
    first_press TEXT    NOT NULL DEFAULT '',
    last_press  TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`

// KeyDelta 是一次 flush 中某个键在某天某小时的增量。
type KeyDelta struct {
	Day   string
	Hour  int
	VK    uint16
	Count int64
}

// ComboDelta 是一次 flush 中某个组合键在某天某小时的增量。
type ComboDelta struct {
	Day   string
	Hour  int
	Combo string
	Count int64
}

// Store 是统计库的句柄，非并发安全，由调用方串行使用。
type Store struct {
	db *sql.DB
}

// Open 打开或创建统计库，并保证表结构就绪。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打开数据库 %s: %w", path, err)
	}

	// 写入量很小，单连接就够，还能避免 WAL 下的写锁竞争
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 WAL 模式: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 busy_timeout: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("建表: %w", err)
	}

	s := &Store{db: db}
	if err := s.SetMeta("schema_version", SchemaVersion); err != nil {
		db.Close()
		return nil, fmt.Errorf("写入 schema_version: %w", err)
	}
	return s, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("关闭数据库: %w", err)
	}
	return nil
}

// AddKeyCounts 批量累加按键计数，同一 (day, hour, vk) 会叠加而不是覆盖。
func (s *Store) AddKeyCounts(deltas []KeyDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO key_count (day, hour, vk, count) VALUES (?, ?, ?, ?)
		ON CONFLICT(day, hour, vk) DO UPDATE SET count = count + excluded.count
	`)
	if err != nil {
		return fmt.Errorf("准备按键 upsert 语句: %w", err)
	}
	defer stmt.Close()

	for _, d := range deltas {
		if _, err := stmt.Exec(d.Day, d.Hour, int64(d.VK), d.Count); err != nil {
			return fmt.Errorf("写入按键 %d: %w", d.VK, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交按键事务: %w", err)
	}
	return nil
}

// AddComboCounts 批量累加组合键计数。
func (s *Store) AddComboCounts(deltas []ComboDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO combo_count (day, hour, combo, count) VALUES (?, ?, ?, ?)
		ON CONFLICT(day, hour, combo) DO UPDATE SET count = count + excluded.count
	`)
	if err != nil {
		return fmt.Errorf("准备组合键 upsert 语句: %w", err)
	}
	defer stmt.Close()

	for _, d := range deltas {
		if _, err := stmt.Exec(d.Day, d.Hour, d.Combo, d.Count); err != nil {
			return fmt.Errorf("写入组合键 %q: %w", d.Combo, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交组合键事务: %w", err)
	}
	return nil
}

// AddActivity 累加某天的活跃秒数，并把首次/最后按键时间收缩到真实极值。
func (s *Store) AddActivity(day string, sec int64, firstPress, lastPress string) error {
	_, err := s.db.Exec(`
		INSERT INTO day_activity (day, active_sec, first_press, last_press) VALUES (?, ?, ?, ?)
		ON CONFLICT(day) DO UPDATE SET
			active_sec = active_sec + excluded.active_sec,
			first_press = CASE
				WHEN first_press = '' THEN excluded.first_press
				WHEN excluded.first_press = '' THEN first_press
				WHEN excluded.first_press < first_press THEN excluded.first_press
				ELSE first_press
			END,
			last_press = CASE
				WHEN excluded.last_press > last_press THEN excluded.last_press
				ELSE last_press
			END
	`, day, sec, firstPress, lastPress)
	if err != nil {
		return fmt.Errorf("写入活跃时长 %s: %w", day, err)
	}
	return nil
}

// KeyCounts 返回闭区间内每个键的总次数。
func (s *Store) KeyCounts(from, to string) (map[uint16]int64, error) {
	rows, err := s.db.Query(
		`SELECT vk, SUM(count) FROM key_count WHERE day BETWEEN ? AND ? GROUP BY vk`, from, to)
	if err != nil {
		return nil, fmt.Errorf("查询按键计数: %w", err)
	}
	defer rows.Close()

	out := make(map[uint16]int64)
	for rows.Next() {
		var vk int64
		var count int64
		if err := rows.Scan(&vk, &count); err != nil {
			return nil, fmt.Errorf("扫描按键行: %w", err)
		}
		out[uint16(vk)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历按键行: %w", err)
	}
	return out, nil
}

// ComboCounts 返回闭区间内每个组合键的总次数。
func (s *Store) ComboCounts(from, to string) (map[string]int64, error) {
	rows, err := s.db.Query(
		`SELECT combo, SUM(count) FROM combo_count WHERE day BETWEEN ? AND ? GROUP BY combo`, from, to)
	if err != nil {
		return nil, fmt.Errorf("查询组合键计数: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var combo string
		var count int64
		if err := rows.Scan(&combo, &count); err != nil {
			return nil, fmt.Errorf("扫描组合键行: %w", err)
		}
		out[combo] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历组合键行: %w", err)
	}
	return out, nil
}

// HourCounts 返回闭区间内按小时聚合的按键次数，交给 stats.HourlyDistribution 补零。
func (s *Store) HourCounts(from, to string) ([]stats.HourCount, error) {
	rows, err := s.db.Query(
		`SELECT hour, SUM(count) FROM key_count WHERE day BETWEEN ? AND ? GROUP BY hour ORDER BY hour`,
		from, to)
	if err != nil {
		return nil, fmt.Errorf("查询时段分布: %w", err)
	}
	defer rows.Close()

	var out []stats.HourCount
	for rows.Next() {
		var h int
		var count int64
		if err := rows.Scan(&h, &count); err != nil {
			return nil, fmt.Errorf("扫描时段行: %w", err)
		}
		out = append(out, stats.HourCount{Hour: h, Count: count})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历时段行: %w", err)
	}
	return out, nil
}

// DailyTotals 返回闭区间内每天的按键总数，用于峰值与趋势。
func (s *Store) DailyTotals(from, to string) (map[string]int64, error) {
	rows, err := s.db.Query(
		`SELECT day, SUM(count) FROM key_count WHERE day BETWEEN ? AND ? GROUP BY day`, from, to)
	if err != nil {
		return nil, fmt.Errorf("查询每日总量: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var day string
		var count int64
		if err := rows.Scan(&day, &count); err != nil {
			return nil, fmt.Errorf("扫描每日行: %w", err)
		}
		out[day] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历每日行: %w", err)
	}
	return out, nil
}

// ActiveSeconds 返回闭区间内的活跃总秒数。
func (s *Store) ActiveSeconds(from, to string) (int64, error) {
	var total sql.NullInt64
	err := s.db.QueryRow(
		`SELECT SUM(active_sec) FROM day_activity WHERE day BETWEEN ? AND ?`, from, to).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("查询活跃时长: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

// PressBounds 返回闭区间内最早的首次按键时间与最晚的最后按键时间（HH:MM:SS）。
func (s *Store) PressBounds(from, to string) (first, last string, err error) {
	var f, l sql.NullString
	err = s.db.QueryRow(
		`SELECT MIN(NULLIF(first_press, '')), MAX(last_press) FROM day_activity WHERE day BETWEEN ? AND ?`,
		from, to).Scan(&f, &l)
	if err != nil {
		return "", "", fmt.Errorf("查询按键时间范围: %w", err)
	}
	return f.String, l.String, nil
}

// FirstDay 返回库中最早有按键数据的日期，空库返回空字符串（「全部」范围的起点）。
func (s *Store) FirstDay() (string, error) {
	var day sql.NullString
	if err := s.db.QueryRow(`SELECT MIN(day) FROM key_count`).Scan(&day); err != nil {
		return "", fmt.Errorf("查询最早日期: %w", err)
	}
	return day.String, nil
}

// ClearAll 清空所有统计数据，但保留 meta（采集开关、引导标记不该被一起清掉）。
func (s *Store) ClearAll() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务: %w", err)
	}
	defer tx.Rollback()

	for _, table := range []string{"key_count", "combo_count", "day_activity"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("清空 %s: %w", table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交清空事务: %w", err)
	}
	return nil
}

// Meta 读取一条元数据，不存在时 ok 为 false。
func (s *Store) Meta(key string) (value string, ok bool, err error) {
	err = s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("读取 meta %q: %w", key, err)
	}
	return value, true, nil
}

// SetMeta 写入或覆盖一条元数据。
func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("写入 meta %q: %w", key, err)
	}
	return nil
}

package service

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"keyboard-statistics/internal/stats"
)

// utf8BOM 让 Excel 直接双击打开时按 UTF-8 解析，不然中文会乱码。
const utf8BOM = "\ufeff"

// ExportCSV 把全部数据导出成两个 CSV（按键 + 组合键），返回生成的文件路径。
//
// 导出的是明细而不是聚合结果，用户拿 Excel 自己能算出任何想要的维度。
func (s *Service) ExportCSV(dir string) ([]string, error) {
	from, err := s.db.FirstDay()
	if err != nil {
		return nil, err
	}
	if from == "" {
		from = s.now().Format(stats.DayLayout)
	}
	to := s.now().Format(stats.DayLayout)

	keyRows, err := s.db.KeyRows(from, to)
	if err != nil {
		return nil, err
	}
	comboRows, err := s.db.ComboRows(from, to)
	if err != nil {
		return nil, err
	}

	prefix := fmt.Sprintf("keyboard-stats-%s-%s", from, to)
	keyFile := filepath.Join(dir, prefix+"_keys.csv")
	comboFile := filepath.Join(dir, prefix+"_combos.csv")

	if err := writeCSV(keyFile, []string{"日期", "小时", "键码", "键名", "次数"},
		func(w *csv.Writer) error {
			for _, r := range keyRows {
				rec := []string{r.Day, strconv.Itoa(r.Hour), strconv.FormatUint(uint64(r.VK), 10), stats.KeyName(r.VK), strconv.FormatInt(r.Count, 10)}
				if err := w.Write(rec); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
		return nil, err
	}

	if err := writeCSV(comboFile, []string{"日期", "小时", "组合", "次数"},
		func(w *csv.Writer) error {
			for _, r := range comboRows {
				rec := []string{r.Day, strconv.Itoa(r.Hour), r.Combo, strconv.FormatInt(r.Count, 10)}
				if err := w.Write(rec); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
		return nil, err
	}

	return []string{keyFile, comboFile}, nil
}

func writeCSV(path string, header []string, body func(*csv.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建导出文件 %s: %w", path, err)
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	if _, err := bw.WriteString(utf8BOM); err != nil {
		return fmt.Errorf("写入 BOM: %w", err)
	}

	w := csv.NewWriter(bw)
	if err := w.Write(header); err != nil {
		return fmt.Errorf("写入表头: %w", err)
	}
	if err := body(w); err != nil {
		return err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("写入 CSV 内容: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("刷盘: %w", err)
	}
	return nil
}

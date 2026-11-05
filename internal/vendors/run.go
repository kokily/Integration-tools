package vendors

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"integration-tools/internal/config"
	"integration-tools/internal/esp"
	"integration-tools/internal/report"
)

// Run: ESP 기간별구매현황 → 거래처별 시트 (웰스토리 제외).
// outRoot에 "거래처별_2026-10.xlsx"로 저장한다
func Run(srcPath, outRoot string, cfg *config.Config) (*report.Report, error) {
	rep := &report.Report{OutDir: outRoot}

	rows, err := esp.Read(srcPath)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("원본에 품목이 없습니다")
	}

	sheets := buildVendorSheets(rows, cfg.Vendors)
	for i, s := range sheets {
		var total float64
		for _, r := range s.Rows {
			total += r.Total
		}
		rep.Linef("%2d. %-12s %3d건  합계금액 %.0f", i+1, s.Name, len(s.Rows), total)
	}

	// 파일 이름의 연월: 원본 첫 줄 입고일자 기준
	month := rows[0].Date
	if len(month) >= 7 {
		month = month[:7]
	}
	if err := os.MkdirAll(outRoot, 0755); err != nil {
		return nil, err
	}
	out := filepath.Join(outRoot, "거래처별_"+month+".xlsx")
	if err := writeVendorFile(out, sheets); err != nil {
		return nil, fmt.Errorf("저장 실패: %w", err)
	}
	rep.Files = append(rep.Files, out)
	rep.Linef("저장: %s (시트 %d개)", filepath.Base(out), len(sheets))
	return rep, nil
}

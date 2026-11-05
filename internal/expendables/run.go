package expendables

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"integration-tools/internal/config"
	"integration-tools/internal/report"
)

// Run: ESP 현재고현황(창고별) → 소모품 결산 (지난 결과 파일을 양식으로 채움).
// outRoot 아래에 "소모품결산_2026-11-05"(실행한 날짜) 폴더를 만들어, 양식과 같은 이름으로 저장한다
func Run(srcPath, templatePath, outRoot string, cfg *config.Config) (*report.Report, error) {
	rep := &report.Report{}

	rows, err := readInventory(srcPath)
	if err != nil {
		return nil, err
	}

	data, warnings := buildExpendables(rows, cfg.Expendables)
	rep.Warnings = append(rep.Warnings, warnings...)

	addExpendablesFixes(rep, rows, cfg.Expendables)

	rep.OutDir = filepath.Join(outRoot, "소모품결산_"+time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(rep.OutDir, 0755); err != nil {
		return nil, err
	}
	out := filepath.Join(rep.OutDir, filepath.Base(templatePath))

	tplWarnings, err := fillExpendablesTemplate(templatePath, out, data, cfg.Expendables)
	if err != nil {
		return nil, fmt.Errorf("저장 실패: %w", err)
	}
	rep.Warnings = append(rep.Warnings, tplWarnings...)
	rep.Files = append(rep.Files, out)

	var total int64
	count := 0
	for _, rs := range data {
		for _, r := range rs {
			total += r.Amount
		}
		count += len(rs)
	}
	rep.Linef("원본 %d줄 → 결산 품목 %d개, 재고금액 합계 %d", len(rows), count, total)
	rep.Linef("저장: %s", filepath.Base(out))
	return rep, nil
}

// addExpendablesFixes: 소모품에서 설정에 없는 창고와 낙찰률 없는 연도를 [설정에 추가] 대상으로 기록
// (buildExpendables가 경고 문구를 만드는 기준과 같음)
func addExpendablesFixes(rep *report.Report, rows []InventoryRow, cfg config.ExpendablesConfig) {
	sheetOf := map[string]string{}
	for sheet, whs := range cfg.Warehouses {
		for _, w := range whs {
			sheetOf[w] = sheet
		}
	}
	excluded := map[string]bool{}
	for _, w := range cfg.Exclude {
		excluded[w] = true
	}
	for _, it := range rows {
		if excluded[it.Warehouse] || (it.Opening == 0 && it.In == 0 && it.Out == 0 && it.Stock == 0) {
			continue
		}
		sheet, ok := sheetOf[it.Warehouse]
		if !ok {
			rep.AddFix("warehouse", it.Warehouse)
			continue
		}
		if _, missing := bidRate(sheet, it.Code, cfg.Rates); missing != "" {
			rep.AddFix("rate", missing)
		}
	}
}

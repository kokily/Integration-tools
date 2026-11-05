package settlement

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"integration-tools/internal/config"
	"integration-tools/internal/esp"
	"integration-tools/internal/report"
)

// Run: ESP 기간별구매현황(계약/비계약) → 결산서 붙여넣기용 파일 + (양식 폴더가 있으면) 결산자료 4개.
// outRoot 아래에 "결산서_2026-10" 폴더를 만들어 저장한다
func Run(contractPath, nonContractPath, templateDir, outRoot string, cfg *config.Config) (*report.Report, error) {
	rep := &report.Report{}

	contract, err := esp.Read(contractPath)
	if err != nil {
		return nil, fmt.Errorf("계약 원본: %w", err)
	}
	nonContract, err := esp.Read(nonContractPath)
	if err != nil {
		return nil, fmt.Errorf("비계약 원본: %w", err)
	}
	if len(contract) == 0 {
		return nil, errors.New("계약 원본에 품목이 없습니다")
	}

	sheets, err := buildSettlement(contract, nonContract, cfg.Settlement)
	if err != nil {
		return nil, err
	}
	for _, s := range sheets {
		var sum float64
		for _, r := range s.Rows {
			sum += r.Amount
		}
		rep.Linef("%-6s %4d건  금액 합계 %.0f", s.Name, len(s.Rows), sum)
	}

	// 결산 월: 계약 원본 첫 줄 입고일자 "2026-10-01" 기준
	first, err := time.Parse("2006-01-02", contract[0].Date)
	if err != nil {
		return nil, fmt.Errorf("입고일자 형식 오류: %q", contract[0].Date)
	}
	lastDay := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.Local).Day() // 다음 달 0일 = 이번 달 마지막 날

	rep.OutDir = filepath.Join(outRoot, "결산서_"+first.Format("2006-01"))
	if err := os.MkdirAll(rep.OutDir, 0755); err != nil {
		return nil, err
	}

	paste := filepath.Join(rep.OutDir, "결산서_붙여넣기.xlsx")
	if err := writeSettlementPasteFile(paste, sheets); err != nil {
		return nil, fmt.Errorf("붙여넣기용 파일 저장 실패: %w", err)
	}
	rep.Files = append(rep.Files, paste)
	rep.Linef("저장: %s", filepath.Base(paste))

	// 양식 폴더를 줬으면 양식 4개도 채움
	if templateDir == "" {
		return rep, nil
	}
	for _, s := range sheets {
		tpl, err := findSettlementTemplate(templateDir, s.Name)
		if err != nil {
			return nil, err
		}
		// "26. 10월 결산자료(계약).xlsx" → "26. 11월 결산자료(계약).xlsx"
		name := settlementFileRe.ReplaceAllString(filepath.Base(tpl), first.Format("06")+". "+fmt.Sprint(int(first.Month()))+"월")
		out := filepath.Join(rep.OutDir, name)
		if err := fillSettlementTemplate(tpl, out, s, int(first.Month()), lastDay); err != nil {
			return nil, fmt.Errorf("%s 저장 실패: %w", name, err)
		}
		rep.Files = append(rep.Files, out)
		rep.Linef("저장: %s (%d건)", name, len(s.Rows))
	}
	return rep, nil
}

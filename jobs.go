package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Report: 기능을 실행한 결과. 터미널(main.go)과 화면(web.go)이 같이 쓴다
type Report struct {
	Lines    []string // 진행 내용 (예: "저장: 01_양식뷔페-계약-면.xlsx (31건)")
	Warnings []string // 확인이 필요한 내용 (예: "설정에 없는 사업장: ○○○")
	OutDir   string   // 결과가 저장된 폴더
	Files    []string // 만든 파일 (전체 경로)
}

// Linef: 진행 내용 한 줄 추가 (fmt.Printf와 같은 모양으로 씀)
func (r *Report) Linef(format string, args ...interface{}) {
	r.Lines = append(r.Lines, fmt.Sprintf(format, args...))
}

// Warnf: 경고 한 줄 추가
func (r *Report) Warnf(format string, args ...interface{}) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// RunOrder: 웰스토리 발주서 → 사업장별 EPS 업로드 파일.
// outRoot 아래에 "발주서_20261029" 폴더를 만들어 저장한다
func RunOrder(src, outRoot string, cfg *Config) (*Report, error) {
	rep := &Report{}

	items, err := readOrder(src)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("원본에 품목이 없습니다")
	}

	// 원본에 여러 날짜가 섞여 있는지 확인
	dates := map[string]bool{}
	for _, it := range items {
		dates[it.Date] = true
	}
	if len(dates) > 1 {
		rep.Warnf("원본에 날짜가 %d개 섞여 있습니다. 한 파일에 여러 날짜가 들어갈 수 있습니다.", len(dates))
	}

	groups, warnings := groupOrder(items, cfg.WelstoryOrder)
	rep.Warnings = append(rep.Warnings, warnings...)

	rep.OutDir = filepath.Join(outRoot, "발주서_"+strings.ReplaceAll(items[0].Date, "/", ""))
	if err := os.MkdirAll(rep.OutDir, 0755); err != nil {
		return nil, err
	}

	var fileNames []string
	for i, g := range groups {
		vendor, ok := cfg.WelstoryOrder.VendorName[g.Contract]
		if !ok {
			return nil, fmt.Errorf("설정 vendorName에 '%s' 거래처명이 없습니다", g.Contract)
		}

		name := fmt.Sprintf("%02d_%s.xlsx", i+1, g.Name())
		path := filepath.Join(rep.OutDir, name)
		if err := writeUploadFile(path, g, vendor); err != nil {
			return nil, fmt.Errorf("%s 저장 실패: %w", name, err)
		}
		fileNames = append(fileNames, name)
		rep.Files = append(rep.Files, path)
		rep.Linef("저장: %s (%d건)", name, len(g.Items))
	}

	refPath := filepath.Join(rep.OutDir, "확인용_전체.xlsx")
	if err := writeReferenceFile(refPath, groups, fileNames, cfg.WelstoryOrder.VendorName); err != nil {
		return nil, fmt.Errorf("확인용 파일 저장 실패: %w", err)
	}
	rep.Files = append(rep.Files, refPath)
	rep.Linef("완료: 업로드 파일 %d개 + 확인용_전체.xlsx", len(groups))
	return rep, nil
}

// RunSettlement: ESP 기간별구매현황(계약/비계약) → 결산서 붙여넣기용 파일 + (양식 폴더가 있으면) 결산자료 4개.
// outRoot 아래에 "결산서_2026-10" 폴더를 만들어 저장한다
func RunSettlement(contractPath, nonContractPath, templateDir, outRoot string, cfg *Config) (*Report, error) {
	rep := &Report{}

	contract, err := readESP(contractPath)
	if err != nil {
		return nil, fmt.Errorf("계약 원본: %w", err)
	}
	nonContract, err := readESP(nonContractPath)
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

// RunExpendables: ESP 현재고현황(창고별) → 소모품 결산 (지난 결과 파일을 양식으로 채움).
// outRoot 아래에 "소모품결산_2026-11-05"(실행한 날짜) 폴더를 만들어, 양식과 같은 이름으로 저장한다
func RunExpendables(srcPath, templatePath, outRoot string, cfg *Config) (*Report, error) {
	rep := &Report{}

	rows, err := readInventory(srcPath)
	if err != nil {
		return nil, err
	}

	data, warnings := buildExpendables(rows, cfg.Expendables)
	rep.Warnings = append(rep.Warnings, warnings...)

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

// RunVendors: ESP 기간별구매현황 → 거래처별 시트 (웰스토리 제외).
// outRoot에 "거래처별_2026-10.xlsx"로 저장한다
func RunVendors(srcPath, outRoot string, cfg *Config) (*Report, error) {
	rep := &Report{OutDir: outRoot}

	rows, err := readESP(srcPath)
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

package order

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"integration-tools/internal/config"
	"integration-tools/internal/report"
)

// Run: 웰스토리 발주서 → 사업장별 EPS 업로드 파일.
// outRoot 아래에 "발주서_20261029" 폴더를 만들어 저장한다
func Run(src, outRoot string, cfg *config.Config) (*report.Report, error) {
	rep := &report.Report{}

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

	for _, g := range groups { // 설정에 없는 사업장·계약구분은 [설정에 추가] 대상
		if indexOf(cfg.WelstoryOrder.Sites, g.Site) == len(cfg.WelstoryOrder.Sites) {
			rep.AddFix("site", g.Site)
		}
		if indexOf(cfg.WelstoryOrder.Contracts, g.Contract) == len(cfg.WelstoryOrder.Contracts) {
			rep.AddFix("contract", g.Contract)
		}
	}

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

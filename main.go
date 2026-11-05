package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const usage = `사용법:
  go run . 발주서 <웰스토리 원본.xlsx>
  go run . 결산서 <계약.xls> <비계약.xls> [지난달 결산자료 폴더]`

func main() {
	if len(os.Args) < 2 {
		log.Fatal(usage)
	}

	cfg, err := loadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}

	// os.Args[1] = 기능 이름, os.Args[2:] = 그 기능에 넘길 파일들
	switch os.Args[1] {
	case "발주서":
		if len(os.Args) != 3 {
			log.Fatal(usage)
		}
		runOrder(os.Args[2], cfg)
	case "결산서":
		if len(os.Args) != 4 && len(os.Args) != 5 {
			log.Fatal(usage)
		}
		templateDir := "" // 양식 폴더는 선택 사항
		if len(os.Args) == 5 {
			templateDir = os.Args[4]
		}
		runSettlement(os.Args[2], os.Args[3], templateDir, cfg)
	default:
		log.Fatalf("알 수 없는 기능: %s\n%s", os.Args[1], usage)
	}
}

// runOrder: 웰스토리 발주서 → 사업장별 EPS 업로드 파일
func runOrder(src string, cfg *Config) {
	items, err := readOrder(src)
	if err != nil {
		log.Fatal(err)
	}
	if len(items) == 0 {
		log.Fatal("원본에 품목이 없습니다")
	}

	// 원본에 여러 날짜가 섞여 있는지 확인
	dates := map[string]bool{}
	for _, it := range items {
		dates[it.Date] = true
	}
	if len(dates) > 1 {
		fmt.Printf("⚠ 원본에 날짜가 %d개 섞여 있습니다. 한 파일에 여러 날짜가 들어갈 수 있습니다.\n", len(dates))
	}

	groups, warnings := groupOrder(items, cfg.WelstoryOrder)
	for _, w := range warnings {
		fmt.Println("⚠", w)
	}

	// 결과 폴더: 원본 옆에 "발주서_20261029"
	outDir := filepath.Join(filepath.Dir(src), "발주서_"+strings.ReplaceAll(items[0].Date, "/", ""))
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatal(err)
	}

	var fileNames []string
	for i, g := range groups {
		vendor, ok := cfg.WelstoryOrder.VendorName[g.Contract]
		if !ok {
			log.Fatalf("설정 vendorName에 '%s' 거래처명이 없습니다", g.Contract)
		}

		name := fmt.Sprintf("%02d_%s.xlsx", i+1, g.Name())
		if err := writeUploadFile(filepath.Join(outDir, name), g, vendor); err != nil {
			log.Fatalf("%s 저장 실패: %v", name, err)
		}
		fileNames = append(fileNames, name)
		fmt.Printf("저장: %s (%d건)\n", name, len(g.Items))
	}

	if err := writeReferenceFile(filepath.Join(outDir, "확인용_전체.xlsx"), groups, fileNames, cfg.WelstoryOrder.VendorName); err != nil {
		log.Fatalf("확인용 파일 저장 실패: %v", err)
	}

	fmt.Printf("\n완료: %s 에 업로드 파일 %d개 + 확인용_전체.xlsx\n", outDir, len(groups))
}

// runSettlement: ESP 기간별구매현황(계약/비계약) → 결산서 붙여넣기용 파일 + (양식 폴더가 있으면) 결산자료 4개
func runSettlement(contractPath, nonContractPath, templateDir string, cfg *Config) {
	contract, err := readESP(contractPath)
	if err != nil {
		log.Fatalf("%s: %v", contractPath, err)
	}
	nonContract, err := readESP(nonContractPath)
	if err != nil {
		log.Fatalf("%s: %v", nonContractPath, err)
	}
	if len(contract) == 0 {
		log.Fatal("계약 원본에 품목이 없습니다")
	}

	sheets, err := buildSettlement(contract, nonContract, cfg.Settlement)
	if err != nil {
		log.Fatal(err)
	}

	for _, s := range sheets {
		var sum float64
		for _, r := range s.Rows {
			sum += r.Amount
		}
		fmt.Printf("%-6s %4d건  금액 합계 %.0f\n", s.Name, len(s.Rows), sum)
	}

	// 결산 월: 계약 원본 첫 줄 입고일자 "2026-10-01" 기준
	first, err := time.Parse("2006-01-02", contract[0].Date)
	if err != nil {
		log.Fatalf("입고일자 형식 오류: %q", contract[0].Date)
	}
	lastDay := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.Local).Day() // 다음 달 0일 = 이번 달 마지막 날

	// 결과 폴더: 계약 원본 옆에 "결산서_2026-10"
	outDir := filepath.Join(filepath.Dir(contractPath), "결산서_"+first.Format("2006-01"))
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatal(err)
	}

	paste := filepath.Join(outDir, "결산서_붙여넣기.xlsx")
	if err := writeSettlementPasteFile(paste, sheets); err != nil {
		log.Fatalf("저장 실패: %v", err)
	}
	fmt.Printf("\n저장: %s\n", paste)

	// 양식 폴더를 줬으면 양식 4개도 채움
	if templateDir == "" {
		return
	}
	for _, s := range sheets {
		tpl, err := findSettlementTemplate(templateDir, s.Name)
		if err != nil {
			log.Fatal(err)
		}
		// "26. 10월 결산자료(계약).xlsx" → "26. 11월 결산자료(계약).xlsx"
		name := settlementFileRe.ReplaceAllString(filepath.Base(tpl), first.Format("06")+". "+fmt.Sprint(int(first.Month()))+"월")
		out := filepath.Join(outDir, name)
		if err := fillSettlementTemplate(tpl, out, s, int(first.Month()), lastDay); err != nil {
			log.Fatalf("%s 저장 실패: %v", name, err)
		}
		fmt.Printf("저장: %s (%d건)\n", out, len(s.Rows))
	}
}

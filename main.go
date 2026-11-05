package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const usage = `사용법:
  go run . 발주서 <웰스토리 원본.xlsx>
  go run . 결산서 <계약.xls> <비계약.xls>`

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
		if len(os.Args) != 4 {
			log.Fatal(usage)
		}
		runSettlement(os.Args[2], os.Args[3], cfg)
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

// runSettlement: ESP 기간별구매현황(계약/비계약) → 결산서 붙여넣기용 파일
func runSettlement(contractPath, nonContractPath string, cfg *Config) {
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

	// 파일 이름의 월: 계약 원본 첫 줄 입고일자 "2026-10-01" → "2026-10"
	month := contract[0].Date[:7]
	out := filepath.Join(filepath.Dir(contractPath), "결산서_붙여넣기_"+month+".xlsx")
	if err := writeSettlementPasteFile(out, sheets); err != nil {
		log.Fatalf("저장 실패: %v", err)
	}
	fmt.Printf("\n완료: %s\n", out)
}

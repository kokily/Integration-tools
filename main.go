package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("사용법: go run . <원본파일.xlsx>")
	}
	src := os.Args[1]

	cfg, err := loadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}

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

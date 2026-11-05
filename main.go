package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"integration-tools/internal/config"
	"integration-tools/internal/expendables"
	"integration-tools/internal/order"
	"integration-tools/internal/report"
	"integration-tools/internal/settlement"
	"integration-tools/internal/vendors"
	"integration-tools/internal/web"
)

// 빌드할 때의 config.json을 exe 안에 같이 넣는다. 다른 PC에서 처음 실행하면 이것으로 설정 파일을 만듦
//
//go:embed config.json
var defaultConfig []byte

const usage = `사용법:
  go run . 발주서 <웰스토리 원본.xlsx>
  go run . 결산서 <계약.xls> <비계약.xls> [지난달 결산자료 폴더]
  go run . 소모품 <현재고현황(창고별).xls> <지난 소모품 결산 파일.xlsx>
  go run . 거래처별 <기간별구매현황[일자별].xls>`

// main: 기능 이름 없이 실행하면(exe 더블클릭) 화면 모드.
// 기능 이름을 주면 터미널 모드 (결과는 원본 파일이 있는 폴더에 저장)
func main() {
	if len(os.Args) < 2 {
		web.Run(defaultConfig)
		return
	}

	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatal(err)
	}

	args := os.Args[2:] // 기능 이름 뒤에 오는 파일들
	var rep *report.Report

	switch os.Args[1] {
	case "발주서":
		if len(args) != 1 {
			log.Fatal(usage)
		}
		rep, err = order.Run(args[0], filepath.Dir(args[0]), cfg)
	case "결산서":
		if len(args) != 2 && len(args) != 3 {
			log.Fatal(usage)
		}
		templateDir := "" // 양식 폴더는 선택 사항
		if len(args) == 3 {
			templateDir = args[2]
		}
		rep, err = settlement.Run(args[0], args[1], templateDir, filepath.Dir(args[0]), cfg)
	case "소모품":
		if len(args) != 2 {
			log.Fatal(usage)
		}
		rep, err = expendables.Run(args[0], args[1], filepath.Dir(args[0]), cfg)
	case "거래처별":
		if len(args) != 1 {
			log.Fatal(usage)
		}
		rep, err = vendors.Run(args[0], filepath.Dir(args[0]), cfg)
	default:
		log.Fatalf("알 수 없는 기능: %s\n%s", os.Args[1], usage)
	}

	if err != nil {
		log.Fatal(err)
	}
	printReport(rep)
}

// printReport: 실행 결과를 터미널에 출력
func printReport(rep *report.Report) {
	for _, w := range rep.Warnings {
		fmt.Println("⚠", w)
	}
	for _, line := range rep.Lines {
		fmt.Println(line)
	}
	fmt.Printf("\n결과 폴더: %s\n", rep.OutDir)
}

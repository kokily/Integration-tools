package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// OrderItem: 원본 발주서의 품목 한 줄
type OrderItem struct {
	Site     string  // 사업장 (예: 양식뷔페)
	Contract string  // 계약 / 비계약
	Date     string  // 일자 (예: 2026/10/29)
	Code     string  // 품목코드 (연도 붙이기 전)
	Name     string  // 품목명
	Spec     string  // 규격
	Unit     string  // 단위
	Qty      float64 // 수량
	Price    float64 // 단가
	Amount   float64 // 입고금액 (공급가액)
	VAT      float64 // 부가세
}

// 원본에 반드시 있어야 하는 열 이름
var requiredCols = []string{"사업장명", "일자", "품목코드", "품목명", "규격", "단위", "수량", "단가", "입고금액", "부가세"}

// "국방컨벤션(양식뷔페/계약)" → 괄호 안의 "양식뷔페", "계약"을 꺼내는 정규식
var siteRe = regexp.MustCompile(`\(([^/()]+)/([^/()]+)\)\s*$`)

// cell: 행의 i번째 칸 값. 행이 짧아서 칸이 없으면 "" 반환
func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// parseNumber: "1,145,300" 같은 문자열을 숫자로. 빈 칸은 0
func parseNumber(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

// findHeader: "사업장명"과 "품목코드"가 함께 있는 행을 헤더로 보고,
// 열 이름 → 열 번호 표(map)를 만든다
func findHeader(rows [][]string) (int, map[string]int, error) {
	for r, row := range rows {
		cols := map[string]int{}
		for c, v := range row {
			cols[strings.TrimSpace(v)] = c
		}
		_, hasSite := cols["사업장명"]
		_, hasCode := cols["품목코드"]
		if !hasSite || !hasCode {
			continue
		}
		for _, name := range requiredCols {
			if _, ok := cols[name]; !ok {
				return 0, nil, fmt.Errorf("%d행 헤더에 '%s' 열이 없습니다", r+1, name)
			}
		}
		return r, cols, nil
	}
	return 0, nil, fmt.Errorf("헤더 행(사업장명, 품목코드)을 찾을 수 없습니다")
}

// readOrder: 웰스토리 발주서 원본을 읽어서 품목 목록으로 만든다
func readOrder(path string) ([]OrderItem, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetName(0), excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, err
	}

	headerRow, col, err := findHeader(rows)
	if err != nil {
		return nil, err
	}

	var items []OrderItem
	var curSite, curDate string // 병합 셀 때문에 비어 있는 줄은 이 값을 이어받음

	for r := headerRow + 1; r < len(rows); r++ {
		row := rows[r]

		// 품목코드가 없으면 소계/합계/페이지번호 줄 → 건너뜀
		code := cell(row, col["품목코드"])
		if code == "" {
			continue
		}

		// 사업장명·일자가 있으면 새 그룹 시작, 없으면 위의 값 유지
		if v := cell(row, col["사업장명"]); v != "" {
			curSite = v
		}
		if v := cell(row, col["일자"]); v != "" {
			curDate = v
		}

		m := siteRe.FindStringSubmatch(curSite)
		if m == nil {
			return nil, fmt.Errorf("%d행: 사업장명 형식을 알 수 없습니다: %q", r+1, curSite)
		}

		item := OrderItem{
			Site:     m[1],
			Contract: m[2],
			Date:     curDate,
			Code:     code,
			Name:     cell(row, col["품목명"]),
			Spec:     cell(row, col["규격"]),
			Unit:     cell(row, col["단위"]),
		}

		// 숫자 칸 4개를 한 번에 처리 (값을 넣을 변수의 주소 &item.Qty 등을 넘김)
		nums := []struct {
			name string
			dst  *float64
		}{
			{"수량", &item.Qty},
			{"단가", &item.Price},
			{"입고금액", &item.Amount},
			{"부가세", &item.VAT},
		}
		for _, n := range nums {
			v, err := parseNumber(cell(row, col[n.name]))
			if err != nil {
				return nil, fmt.Errorf("%d행 %s 값이 숫자가 아닙니다: %q", r+1, n.name, cell(row, col[n.name]))
			}
			*n.dst = v
		}

		items = append(items, item)
	}
	return items, nil
}

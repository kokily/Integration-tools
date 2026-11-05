package expendables

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"integration-tools/internal/config"
	"integration-tools/internal/esp"
	"integration-tools/internal/excel"
)

// InventoryRow: ESP 현재고현황(창고별)의 품목 한 줄
type InventoryRow struct {
	Warehouse string  // 창고 (원본 이름)
	Code      string  // 품번
	Name      string  // 품명
	Spec      string  // 규격
	Unit      string  // 단위
	Opening   float64 // 기초수량 → 결과의 이월수량
	In        float64 // 입고수량
	Out       float64 // 출고수량
	Stock     float64 // 재고수량
	Price     float64 // ①추정금액의 재고단가
}

var inventoryRequiredCols = []string{"창고", "품번", "품명", "규격", "단위", "기초수량", "입고수량", "출고수량", "재고수량", "①추정금액"}

// readInventory: ESP 현재고현황(창고별) HTML 파일을 읽는다
func readInventory(path string) ([]InventoryRow, error) {
	rows, err := esp.ReadTable(path)
	if err != nil {
		return nil, err
	}

	headerRow := -1
	var col map[string]int
	for r, row := range rows {
		m := map[string]int{}
		for c, v := range row {
			if v != "" {
				m[v] = c
			}
		}
		found := true
		for _, name := range inventoryRequiredCols {
			if _, ok := m[name]; !ok {
				found = false
				break
			}
		}
		if found {
			headerRow, col = r, m
			break
		}
	}
	if headerRow < 0 {
		return nil, fmt.Errorf("헤더 행을 찾을 수 없습니다. ESP 현재고현황(창고별) 파일이 맞나요?")
	}

	var result []InventoryRow
	for r := headerRow + 1; r < len(rows); r++ {
		row := rows[r]
		code := excel.Cell(row, col["품번"])
		if code == "" { // 헤더 둘째 줄(재고단가/재고금액), [ 합 계 ] 줄
			continue
		}
		it := InventoryRow{
			Warehouse: excel.Cell(row, col["창고"]),
			Code:      code,
			Name:      excel.Cell(row, col["품명"]),
			Spec:      excel.Cell(row, col["규격"]),
			Unit:      excel.Cell(row, col["단위"]),
		}
		nums := []struct {
			col int
			dst *float64
		}{
			{col["기초수량"], &it.Opening},
			{col["입고수량"], &it.In},
			{col["출고수량"], &it.Out},
			{col["재고수량"], &it.Stock},
			{col["①추정금액"], &it.Price}, // ①추정금액 아래 첫 칸이 재고단가
		}
		for _, n := range nums {
			v, err := excel.ParseNumber(excel.Cell(row, n.col))
			if err != nil {
				return nil, fmt.Errorf("%d행 숫자 오류: %q", r+1, excel.Cell(row, n.col))
			}
			*n.dst = v
		}
		result = append(result, it)
	}
	return result, nil
}

// roundByFive: 1의 자리가 5 이상이면 올리고, 미만이면 버려서 10원 단위로 (기존 앱 규칙)
func roundByFive(n int64) int64 {
	m := n % 10
	switch {
	case m == 0:
		return n
	case m < 5:
		return n - m
	default:
		return n - m + 10
	}
}

// bidRate: (비) 시트 품목의 낙찰률. 품번이 "연도2자리+1"(예: 261...)로 시작할 때만 적용
func bidRate(sheet, code string, rates map[string]float64) (rate float64, missingYear string) {
	if !strings.HasSuffix(sheet, "(비)") || len(code) < 3 || code[2] != '1' {
		return 1, ""
	}
	year := code[:2]
	if r, ok := rates[year]; ok {
		return r, ""
	}
	return 1, year
}

// ExpendableRow: 결과 시트 B~L열 한 줄
type ExpendableRow struct {
	InventoryRow
	Amount int64 // L 재고금액
}

// buildExpendables: 원본을 결과 시트별로 나누고, 정렬하고, 재고금액을 계산한다
func buildExpendables(rows []InventoryRow, cfg config.ExpendablesConfig) (map[string][]ExpendableRow, []string) {
	// 원본 창고 → 결과 시트
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

	result := map[string][]ExpendableRow{}
	unknown := map[string]int{}
	missingRate := map[string]int{}
	var unknownOrder []string

	for _, it := range rows {
		if excluded[it.Warehouse] {
			continue
		}
		// 기초·입고·출고·재고가 모두 0이면 결산에서 뺌
		if it.Opening == 0 && it.In == 0 && it.Out == 0 && it.Stock == 0 {
			continue
		}
		sheet, ok := sheetOf[it.Warehouse]
		if !ok {
			if unknown[it.Warehouse] == 0 {
				unknownOrder = append(unknownOrder, it.Warehouse)
			}
			unknown[it.Warehouse]++
			continue
		}
		rate, missing := bidRate(sheet, it.Code, cfg.Rates)
		if missing != "" {
			missingRate[missing]++
		}
		amount := roundByFive(int64(math.Round(it.Price * rate * it.Stock)))
		result[sheet] = append(result[sheet], ExpendableRow{InventoryRow: it, Amount: amount})
	}

	for sheet := range result {
		rs := result[sheet]
		sort.SliceStable(rs, func(i, j int) bool { return excel.LessKorean(rs[i].Name, rs[j].Name) })
	}

	var warnings []string
	for _, w := range unknownOrder {
		warnings = append(warnings, fmt.Sprintf("설정에 없는 창고: %s (품목 %d개) → 결산에서 빠짐. 아래 [설정에 추가]나 설정 탭 '창고 매핑'에서 추가하세요", w, unknown[w]))
	}
	for year, n := range missingRate {
		warnings = append(warnings, fmt.Sprintf("%s년 낙찰률이 설정에 없습니다 (품목 %d개는 100%%로 계산). 아래 [설정에 추가]나 설정 탭 '낙찰률'에서 추가하세요", year, n))
	}
	return result, warnings
}

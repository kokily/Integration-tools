package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// VendorSheet: 거래처 시트 하나 분량
type VendorSheet struct {
	Name string        // 거래처명 (시트 이름)
	Rows []PurchaseRow // 원본 순서 그대로
}

// buildVendorSheets: 거래처별로 나누고 시트 순서를 정한다 (설정 순서 → 나머지 가나다순)
func buildVendorSheets(rows []PurchaseRow, cfg VendorsConfig) []VendorSheet {
	excluded := map[string]bool{}
	for _, v := range cfg.Exclude {
		excluded[v] = true
	}

	byVendor := map[string][]PurchaseRow{}
	for _, r := range rows {
		if r.Vendor == "" || excluded[r.Vendor] {
			continue
		}
		byVendor[r.Vendor] = append(byVendor[r.Vendor], r)
	}

	var sheets []VendorSheet
	listed := map[string]bool{}
	for _, v := range cfg.Order {
		listed[v] = true
		sheets = append(sheets, VendorSheet{Name: v, Rows: byVendor[v]}) // 거래가 없어도 빈 시트를 만듦
	}

	var rest []string
	for v := range byVendor {
		if !listed[v] {
			rest = append(rest, v)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return lessKorean(rest[i], rest[j]) })
	for _, v := range rest {
		sheets = append(sheets, VendorSheet{Name: v, Rows: byVendor[v]})
	}
	return sheets
}

// 엑셀 시트 이름에 쓸 수 없는 글자
var sheetNameReplacer = strings.NewReplacer(":", "_", "\\", "_", "/", "_", "?", "_", "*", "_", "[", "_", "]", "_")

// safeSheetName: 시트 이름으로 쓸 수 없는 글자를 바꾸고 31글자로 자름
func safeSheetName(name string) string {
	s := sheetNameReplacer.Replace(strings.TrimSpace(name))
	if utf8.RuneCountInString(s) > 31 {
		s = string([]rune(s)[:31])
	}
	return s
}

var vendorHeaders = []interface{}{
	"순번", "입고일자", "품번", "품명", "규격", "수량", "단위",
	"단가", "금액", "부가세", "합계금액", "거래처명", "현장명", "입고창고",
}

// 숫자 열 (F 수량, H 단가, I 금액, J 부가세, K 합계금액)
var vendorNumberCols = map[int]bool{6: true, 8: true, 9: true, 10: true, 11: true}

// displayWidth: 열 너비 계산용 글자 폭 (한글 등은 2칸)
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r < 128 {
			w++
		} else {
			w += 2
		}
	}
	return w
}

// shortDate: "2026-10-01" → "10월 1일" (형식이 다르면 그대로)
func shortDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return fmt.Sprintf("%d월 %d일", t.Month(), t.Day())
}

// writeVendorFile: 거래처마다 시트를 만들어 저장한다
func writeVendorFile(path string, sheets []VendorSheet) error {
	f := excelize.NewFile()
	defer f.Close()

	textFmt := "@"
	textStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &textFmt})
	if err != nil {
		return err
	}
	intStyle, err := f.NewStyle(&excelize.Style{NumFmt: 3}) // #,##0
	if err != nil {
		return err
	}
	decFmt := "#,##0.0##" // 소수점이 있는 값 (예: 단가 236.5)
	decStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &decFmt})
	if err != nil {
		return err
	}

	used := map[string]bool{}
	for i, s := range sheets {
		name := safeSheetName(s.Name)
		if used[name] {
			return fmt.Errorf("시트 이름이 겹칩니다: %s", name)
		}
		used[name] = true
		if i == 0 {
			if err := f.SetSheetName("Sheet1", name); err != nil {
				return err
			}
		} else if _, err := f.NewSheet(name); err != nil {
			return err
		}

		widths := make([]int, len(vendorHeaders))
		note := func(col int, text string) { // 열 너비용으로 가장 긴 글자 기억
			if w := displayWidth(text); w > widths[col] {
				widths[col] = w
			}
		}

		if err := f.SetSheetRow(name, "A1", &vendorHeaders); err != nil {
			return err
		}
		for c, h := range vendorHeaders {
			note(c, h.(string))
		}

		var sumQty, sumAmount, sumVAT, sumTotal float64
		for r, p := range s.Rows {
			values := []interface{}{
				p.Seq, shortDate(p.Date), p.Code, p.Name, p.Spec, p.Qty, p.Unit,
				p.Price, p.Amount, p.VAT, p.Total, p.Vendor, p.Site, p.Warehouse,
			}
			cellRef, _ := excelize.CoordinatesToCellName(1, r+2)
			if err := f.SetSheetRow(name, cellRef, &values); err != nil {
				return err
			}
			for c, v := range values {
				note(c, fmt.Sprint(v))
			}
			sumQty += p.Qty
			sumAmount += p.Amount
			sumVAT += p.VAT
			sumTotal += p.Total
		}

		// "계": 마지막 데이터 행 아래 두 줄 띄우고 E열부터
		totalRow := len(s.Rows) + 4
		totals := []interface{}{"계", sumQty, nil, nil, sumAmount, sumVAT, sumTotal}
		if err := f.SetSheetRow(name, fmt.Sprintf("E%d", totalRow), &totals); err != nil {
			return err
		}

		// 품목별 요약표: "계" 아래 두 줄 띄우고 (거래가 없는 시트는 만들지 않음)
		lastRow := totalRow
		var decCells []string // 소수점 서식을 줄 요약표 단가 칸
		if len(s.Rows) > 0 {
			end, cells, err := writeVendorSummary(f, name, s.Rows, totalRow+3)
			if err != nil {
				return err
			}
			lastRow, decCells = end, cells
		}

		// 서식: 글자 열은 텍스트, 숫자 열은 천 단위 쉼표 (소수점이 있는 칸만 소수 표시)
		for c := 1; c <= len(vendorHeaders); c++ {
			col, _ := excelize.ColumnNumberToName(c)
			style := textStyle
			if vendorNumberCols[c] {
				style = intStyle
			}
			if err := f.SetCellStyle(name, col+"1", fmt.Sprintf("%s%d", col, lastRow), style); err != nil {
				return err
			}
			width := float64(widths[c-1]) + 2
			f.SetColWidth(name, col, col, math.Max(9, math.Min(width, 60)))
		}
		f.SetCellStyle(name, fmt.Sprintf("E%d", totalRow), fmt.Sprintf("E%d", totalRow), textStyle)
		for r, p := range s.Rows {
			for c, v := range map[int]float64{6: p.Qty, 8: p.Price, 9: p.Amount, 10: p.VAT, 11: p.Total} {
				if v != math.Trunc(v) {
					cellRef, _ := excelize.CoordinatesToCellName(c, r+2)
					f.SetCellStyle(name, cellRef, cellRef, decStyle)
				}
			}
		}
		for _, cellRef := range decCells {
			f.SetCellStyle(name, cellRef, cellRef, decStyle)
		}
	}

	// 요약표 수식을 파일을 열 때 계산하도록 설정
	recalc := true
	if err := f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: &recalc}); err != nil {
		return err
	}
	return f.SaveAs(path)
}

// writeVendorSummary: 거래처 시트 아래쪽에 품목별 요약표를 쓴다 (더브레드뱅크 시트에 직접 걸었던 수식과 같은 모양)
//
//	수량 = SUMIF(위 데이터의 품번, 이 품번, 위 데이터의 수량)
//	금액 = 수량 × 단가,  부가세 = 금액 × 0.1,  합계금액 = 금액 + 부가세
//
// 같은 품번인데 단가가 여러 개면 단가별로 줄을 나누고 수량은 SUMIFS(품번과 단가가 모두 같은 줄)로 구한다.
// 반환값: 요약표 마지막 행(계), 소수점 서식을 줄 단가 칸 목록
func writeVendorSummary(f *excelize.File, sheet string, rows []PurchaseRow, headerRow int) (int, []string, error) {
	dataEnd := len(rows) + 1 // 위 데이터의 마지막 행

	// 품번+단가별로 묶기 (처음 나온 순서). items에는 각 묶음의 첫 줄을 담아 품명·규격·단위·단가를 가져옴
	var items []PurchaseRow
	seen := map[string]bool{}
	prices := map[string]map[float64]bool{} // 품번 → 단가 종류
	for _, r := range rows {
		if prices[r.Code] == nil {
			prices[r.Code] = map[float64]bool{}
		}
		prices[r.Code][r.Price] = true
		key := fmt.Sprintf("%s\x00%v", r.Code, r.Price)
		if !seen[key] {
			seen[key] = true
			items = append(items, r)
		}
	}

	header := []interface{}{"품번", "품명", "규격", "수량", "단위", "단가", "금액", "부가세", "합계금액"}
	if err := f.SetSheetRow(sheet, fmt.Sprintf("C%d", headerRow), &header); err != nil {
		return 0, nil, err
	}

	var decCells []string
	for i, p := range items {
		row := headerRow + 1 + i
		values := []interface{}{p.Code, p.Name, p.Spec, nil, p.Unit, p.Price}
		if err := f.SetSheetRow(sheet, fmt.Sprintf("C%d", row), &values); err != nil {
			return 0, nil, err
		}
		if p.Price != math.Trunc(p.Price) {
			decCells = append(decCells, fmt.Sprintf("H%d", row))
		}

		qty := fmt.Sprintf("SUMIF($C$2:$C$%d,C%d,$F$2:$F$%d)", dataEnd, row, dataEnd)
		if len(prices[p.Code]) > 1 {
			qty = fmt.Sprintf("SUMIFS($F$2:$F$%d,$C$2:$C$%d,C%d,$H$2:$H$%d,H%d)", dataEnd, dataEnd, row, dataEnd, row)
		}
		formulas := map[string]string{
			fmt.Sprintf("F%d", row): qty,
			fmt.Sprintf("I%d", row): fmt.Sprintf("F%d*H%d", row, row),
			fmt.Sprintf("J%d", row): fmt.Sprintf("I%d*0.1", row),
			fmt.Sprintf("K%d", row): fmt.Sprintf("I%d+J%d", row, row),
		}
		for cellRef, fm := range formulas {
			if err := f.SetCellFormula(sheet, cellRef, fm); err != nil {
				return 0, nil, err
			}
		}
	}

	// 요약표 "계": 한 줄 띄우고 E열
	first, last := headerRow+1, headerRow+len(items)
	sumRow := last + 2
	if err := f.SetCellValue(sheet, fmt.Sprintf("E%d", sumRow), "계"); err != nil {
		return 0, nil, err
	}
	for _, col := range []string{"F", "I", "J", "K"} {
		fm := fmt.Sprintf("SUM(%s%d:%s%d)", col, first, col, last)
		if err := f.SetCellFormula(sheet, fmt.Sprintf("%s%d", col, sumRow), fm); err != nil {
			return 0, nil, err
		}
	}
	return sumRow, decCells, nil
}

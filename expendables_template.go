package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xuri/excelize/v2"
)

// expendablesLayout: 창고 시트에서 찾아낸 행 위치
type expendablesLayout struct {
	dataStart int // 첫 데이터 행 ("순번" 헤더 바로 아래)
	totalRow  int // "계" 행
}

// findExpendablesLayout: A열에서 "순번"과 "계"를 찾음. 못 찾으면 ok=false (결산서 요약 시트 등)
func findExpendablesLayout(f *excelize.File, sheet string) (l expendablesLayout, ok bool, err error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return l, false, err
	}
	for i, row := range rows {
		a := ""
		if len(row) > 0 {
			a = strings.ReplaceAll(row[0], " ", "")
		}
		if l.dataStart == 0 && a == "순번" {
			l.dataStart = i + 2
		} else if l.dataStart > 0 && a == "계" {
			l.totalRow = i + 1
			return l, true, nil
		}
	}
	return l, false, nil
}

// 수식 안의 "J4:J700" 같은 같은 열 범위
var rowRangeRe = regexp.MustCompile(`\b([A-Z]{1,3})\$?\d+:([A-Z]{1,3})\$?\d+\b`)

// sheetRange: 한 창고 시트를 채운 뒤의 데이터 범위 (요약 시트 수식에 씀)
type sheetRange struct {
	first, last int // 데이터 첫 행, 마지막 행
	total       int // "계" 행
}

// 창고 시트에서 다루는 열: A(순번) ~ L(재고금액)
const expendablesLastCol = 12

// rowSnapshot: 한 행의 칸별 서식 번호, 값, 수식 (견본으로 기억해 둠)
type rowSnapshot struct {
	styles   []int
	values   []string
	formulas []string
}

func snapshotRow(f *excelize.File, sheet string, row int) (rowSnapshot, error) {
	var s rowSnapshot
	for c := 1; c <= expendablesLastCol; c++ {
		cellRef, _ := excelize.CoordinatesToCellName(c, row)
		style, err := f.GetCellStyle(sheet, cellRef)
		if err != nil {
			return s, err
		}
		value, _ := f.GetCellValue(sheet, cellRef)
		formula, _ := f.GetCellFormula(sheet, cellRef)
		s.styles = append(s.styles, style)
		s.values = append(s.values, value)
		s.formulas = append(s.formulas, formula)
	}
	return s, nil
}

// writeCell: 칸에 서식과 값(또는 수식)을 씀. 원래 있던 수식은 지움
func writeCell(f *excelize.File, sheet string, col, row, style int, value interface{}, formula string) error {
	cellRef, _ := excelize.CoordinatesToCellName(col, row)
	if err := f.SetCellStyle(sheet, cellRef, cellRef, style); err != nil {
		return err
	}
	if formula != "" {
		return f.SetCellFormula(sheet, cellRef, formula)
	}
	if old, _ := f.GetCellFormula(sheet, cellRef); old != "" {
		if err := f.SetCellFormula(sheet, cellRef, ""); err != nil { // 빈 수식 = 수식 지우기
			return err
		}
	}
	return f.SetCellValue(sheet, cellRef, value)
}

// fitExpendablesSheet: 창고 시트를 "데이터 N행 + 빈 줄 1행 + 계 행" 모양으로 다시 쓴다.
// 행을 끼우거나 지우지 않고 제자리에서 다시 쓰므로 빠름 (excelize의 행 삭제는 행마다 시트 전체를 다시 계산해서 느림)
func fitExpendablesSheet(f *excelize.File, sheet string, l expendablesLayout, rows []ExpendableRow) (sheetRange, error) {
	area := l.totalRow - 1 - l.dataStart // 양식의 데이터 행 수 ("계" 위 빈 줄 제외)
	if area < 1 {
		return sheetRange{}, fmt.Errorf("'%s' 시트에 서식 견본이 될 데이터 행이 없습니다", sheet)
	}
	want := len(rows)
	if want < 1 {
		want = 1 // 품목이 없어도 빈 데이터 행 1줄은 남김 (다음에 쓸 서식 견본)
	}

	// 1) 견본 기억: 첫 데이터 행, 나머지 데이터 행, "계" 위 빈 줄, "계" 행
	firstTpl, err := snapshotRow(f, sheet, l.dataStart)
	if err != nil {
		return sheetRange{}, err
	}
	middleTpl := firstTpl
	if area >= 2 {
		if middleTpl, err = snapshotRow(f, sheet, l.dataStart+1); err != nil {
			return sheetRange{}, err
		}
	}
	spareTpl, err := snapshotRow(f, sheet, l.totalRow-1)
	if err != nil {
		return sheetRange{}, err
	}
	totalTpl, err := snapshotRow(f, sheet, l.totalRow)
	if err != nil {
		return sheetRange{}, err
	}

	r := sheetRange{first: l.dataStart, last: l.dataStart + want - 1, total: l.dataStart + want + 1}

	// 2) 데이터 행: A열 순번은 숫자로, B~L열은 값으로
	warehouse := strings.ReplaceAll(sheet, " ", "") // B열 창고 이름 ("양식 별도" 시트 → "양식별도")
	for i := 0; i < want; i++ {
		tpl := middleTpl
		if i == 0 {
			tpl = firstTpl
		}
		values := make([]interface{}, expendablesLastCol) // 데이터가 없는 행은 전부 nil = 빈 칸
		if i < len(rows) {
			x := rows[i]
			values = []interface{}{
				i + 1, warehouse, x.Code, textOrNil(x.Name), textOrNil(x.Spec), textOrNil(x.Unit),
				x.Opening, x.In, x.Out, x.Stock, x.Price, x.Amount,
			}
		}
		for c := 1; c <= expendablesLastCol; c++ {
			if err := writeCell(f, sheet, c, r.first+i, tpl.styles[c-1], values[c-1], ""); err != nil {
				return sheetRange{}, err
			}
		}
	}

	// 3) 빈 줄과 "계" 행을 새 위치에. "계" 수식 범위는 새 데이터 범위로 (예: SUM(J4:J700) → SUM(J4:J124))
	for c := 1; c <= expendablesLastCol; c++ {
		if err := writeCell(f, sheet, c, r.total-1, spareTpl.styles[c-1], textOrNil(spareTpl.values[c-1]), ""); err != nil {
			return sheetRange{}, err
		}
		formula := totalTpl.formulas[c-1]
		if formula != "" {
			formula = rowRangeRe.ReplaceAllString(formula, fmt.Sprintf("${1}%d:${2}%d", r.first, r.last))
		}
		if err := writeCell(f, sheet, c, r.total, totalTpl.styles[c-1], textOrNil(totalTpl.values[c-1]), formula); err != nil {
			return sheetRange{}, err
		}
	}

	// 4) 그 아래 남은 옛 줄은 값·수식·서식을 모두 지움
	if r.total < l.totalRow {
		for row := r.total + 1; row <= l.totalRow; row++ {
			for c := 1; c <= expendablesLastCol; c++ {
				if err := writeCell(f, sheet, c, row, 0, nil, ""); err != nil {
					return sheetRange{}, err
				}
			}
		}
		// 양식의 빈 행에는 "행 전체 서식"이 걸려 있어서, 행 서식도 기본값(0)으로 되돌려야 테두리가 사라짐
		if err := f.SetRowStyle(sheet, r.total+1, l.totalRow, 0); err != nil {
			return sheetRange{}, err
		}
	}

	// 5) 인쇄 영역: 1행 ~ "계" 행
	if err := setPrintArea(f, sheet, fmt.Sprintf("$A$1:$L$%d", r.total)); err != nil {
		return sheetRange{}, err
	}
	return r, nil
}

// setPrintArea: 시트의 인쇄 영역을 바꿈 (기존 것이 있으면 지우고 새로)
func setPrintArea(f *excelize.File, sheet, ref string) error {
	refersTo := "'" + sheet + "'!" + ref
	for _, dn := range f.GetDefinedName() {
		if dn.Name == "_xlnm.Print_Area" && dn.Scope == sheet {
			if err := f.DeleteDefinedName(&excelize.DefinedName{Name: dn.Name, Scope: sheet}); err != nil {
				return err
			}
		}
	}
	return f.SetDefinedName(&excelize.DefinedName{Name: "_xlnm.Print_Area", RefersTo: refersTo, Scope: sheet})
}

// updateSummarySheet: 요약 시트(B열에 창고 시트 이름이 있는 행)의 수식을 실제 범위로 다시 씀
// C~F: 이월/입고/출고/재고수량 합계, G: 재고수량 "계", H: 재고금액 "계"
func updateSummarySheet(f *excelize.File, ranges map[string]sheetRange) error {
	for _, sheet := range f.GetSheetList() {
		if _, isWarehouse := ranges[sheet]; isWarehouse {
			continue // 창고 시트는 건너뜀 (B열에 창고 이름이 있어서 요약 행으로 착각할 수 있음)
		}
		rows, err := f.GetRows(sheet)
		if err != nil {
			return err
		}
		for i, row := range rows {
			if len(row) < 2 {
				continue
			}
			r, ok := ranges[strings.TrimSpace(row[1])]
			if !ok {
				continue
			}
			ref := "'" + strings.TrimSpace(row[1]) + "'!"
			formulas := []string{
				fmt.Sprintf("SUM(%sG%d:G%d)", ref, r.first, r.last),
				fmt.Sprintf("SUM(%sH%d:H%d)", ref, r.first, r.last),
				fmt.Sprintf("SUM(%sI%d:I%d)", ref, r.first, r.last),
				fmt.Sprintf("SUM(%sJ%d:J%d)", ref, r.first, r.last),
				fmt.Sprintf("%sJ%d", ref, r.total),
				fmt.Sprintf("%sL%d", ref, r.total),
			}
			for c, fm := range formulas {
				cellRef, _ := excelize.CoordinatesToCellName(c+3, i+1) // C열부터
				if err := f.SetCellFormula(sheet, cellRef, fm); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// fillExpendablesTemplate: 소모품 결산 양식(지난 결과 파일)을 열어 창고 시트를 품목 수에 맞춰 채우고
// 요약 시트 수식과 인쇄 영역을 맞춘 새 파일을 만든다
func fillExpendablesTemplate(templatePath, outPath string, data map[string][]ExpendableRow, cfg ExpendablesConfig) ([]string, error) {
	f, err := openXLSXNormalized(templatePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var warnings []string
	ranges := map[string]sheetRange{}

	for _, sheet := range f.GetSheetList() {
		l, ok, err := findExpendablesLayout(f, sheet)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue // 순번/계가 없는 시트 (결산서 요약)
		}
		if _, inConfig := cfg.Warehouses[sheet]; !inConfig {
			warnings = append(warnings, fmt.Sprintf("양식의 '%s' 시트는 설정에 없어 비워 둡니다", sheet))
		}
		r, err := fitExpendablesSheet(f, sheet, l, data[sheet]) // 품목이 없는 시트는 nil → 빈 행 1줄
		if err != nil {
			return nil, err
		}
		ranges[sheet] = r
	}

	for sheet := range cfg.Warehouses {
		if _, ok := ranges[sheet]; !ok {
			return nil, fmt.Errorf("양식에 '%s' 시트가 없습니다", sheet)
		}
	}

	if err := updateSummarySheet(f, ranges); err != nil {
		return nil, err
	}

	recalc := true
	if err := f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: &recalc}); err != nil {
		return nil, err
	}
	return warnings, f.SaveAs(outPath)
}

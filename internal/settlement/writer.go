package settlement

import (
	"fmt"

	"github.com/xuri/excelize/v2"

	"integration-tools/internal/excel"
)

// writeSettlementPasteFile: 결과 파일별 시트(A~J열 값만) + 요약 시트
func writeSettlementPasteFile(path string, sheets []SettlementSheet) error {
	f := excelize.NewFile()
	defer f.Close()

	for i, s := range sheets {
		if i == 0 {
			f.SetSheetName("Sheet1", s.Name)
		} else if _, err := f.NewSheet(s.Name); err != nil {
			return err
		}

		for r, row := range s.Rows {
			values := []interface{}{
				r + 1, row.Date, row.Place, excel.TextOrNil(row.Name), excel.TextOrNil(row.Spec),
				row.Tax, excel.TextOrNil(row.Unit), row.Qty, row.Price, row.Amount,
			}
			cellRef, _ := excelize.CoordinatesToCellName(1, r+1)
			if err := f.SetSheetRow(s.Name, cellRef, &values); err != nil {
				return err
			}
		}
		f.SetColWidth(s.Name, "D", "E", 30)
	}

	if err := writeSettlementSummary(f, sheets); err != nil {
		return err
	}
	return f.SaveAs(path)
}

// writeSettlementSummary: 시트별·납품장소별 면세/과세/계 (양식 R~U열 월 종합과 비교용)
func writeSettlementSummary(f *excelize.File, sheets []SettlementSheet) error {
	const name = "요약"
	if _, err := f.NewSheet(name); err != nil {
		return err
	}

	r := 1
	var err error
	// put: 다음 행에 값들을 씀 (중간에 오류가 나면 이후는 건너뜀)
	put := func(values ...interface{}) {
		if err != nil {
			return
		}
		cellRef, _ := excelize.CoordinatesToCellName(1, r)
		err = f.SetSheetRow(name, cellRef, &values)
		r++
	}

	for _, s := range sheets {
		put(fmt.Sprintf("%s (%d건)", s.Name, len(s.Rows)))
		put("납품장소", "면세", "과세", "계")

		var places []string             // 처음 나온 순서
		sums := map[string][2]float64{} // [0]=면세, [1]=과세
		for _, row := range s.Rows {
			v, seen := sums[row.Place]
			if !seen {
				places = append(places, row.Place)
			}
			if row.Tax == "면" {
				v[0] += row.Amount
			} else {
				v[1] += row.Amount
			}
			sums[row.Place] = v
		}

		var total [2]float64
		for _, p := range places {
			v := sums[p]
			put(p, v[0], v[1], v[0]+v[1])
			total[0] += v[0]
			total[1] += v[1]
		}
		put("합계", total[0], total[1], total[0]+total[1])
		r++ // 시트 사이 빈 줄
	}

	f.SetColWidth(name, "A", "A", 16)
	f.SetColWidth(name, "B", "D", 14)
	return err
}

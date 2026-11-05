package order

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

// EPS 업로드 양식 헤더 (칸 이름과 순서가 정확히 같아야 업로드됨)
var uploadHeaders = []interface{}{
	"발주일자", "코드", "거래처명", "품번", "품명", "규격", "바코드",
	"수량", "단가", "공급가액", "부가세액", "적요", "PJT코드", "입고예정일", "관리번호",
}

// 확인용 파일 헤더: 업로드 양식 15칸 + 빈칸 + 단위 + 구분
var referenceHeaders = append(append([]interface{}{}, uploadHeaders...), "", "단위", "구분")

// newDateStyle: 날짜를 2026-10-29 형태로 보이게 하는 스타일
func newDateStyle(f *excelize.File) (int, error) {
	format := "yyyy-mm-dd"
	return f.NewStyle(&excelize.Style{CustomNumFmt: &format})
}

// uploadRow: 품목 하나를 업로드 양식 15칸 행으로 변환
func uploadRow(it OrderItem, vendor string) ([]interface{}, error) {
	date, err := time.Parse("2006/01/02", it.Date)
	if err != nil {
		return nil, fmt.Errorf("일자 형식 오류: %q", it.Date)
	}
	code, err := codeWithYear(it.Date, it.Code)
	if err != nil {
		return nil, err
	}
	// ""  = 빈 문자열이 들어간 칸 (Ctrl+방향키 선택이 끊기지 않음)
	// nil = 진짜 빈 칸 (적요, PJT코드)
	return []interface{}{
		date, "", vendor, code, it.Name, it.Spec, "",
		it.Qty, it.Price, it.Amount, it.VAT, nil, nil, date, "",
	}, nil
}

// applyUploadLayout: 날짜 서식과 열 너비 (업로드/확인용 공통)
func applyUploadLayout(f *excelize.File, sheet string, lastRow int) error {
	dateStyle, err := newDateStyle(f)
	if err != nil {
		return err
	}
	f.SetCellStyle(sheet, "A2", fmt.Sprintf("A%d", lastRow), dateStyle)
	f.SetCellStyle(sheet, "N2", fmt.Sprintf("N%d", lastRow), dateStyle)
	f.SetColWidth(sheet, "A", "A", 12)
	f.SetColWidth(sheet, "D", "D", 15)
	f.SetColWidth(sheet, "E", "F", 30)
	f.SetColWidth(sheet, "N", "N", 12)
	return nil
}

// writeUploadFile: 묶음 하나를 EPS 업로드 양식 파일로 저장
func writeUploadFile(path string, g OrderGroup, vendor string) error {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Sheet1"

	if err := f.SetSheetRow(sheet, "A1", &uploadHeaders); err != nil {
		return err
	}
	for i, it := range g.Items {
		row, err := uploadRow(it, vendor)
		if err != nil {
			return err
		}
		cellRef, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := f.SetSheetRow(sheet, cellRef, &row); err != nil {
			return err
		}
	}
	if err := applyUploadLayout(f, sheet, len(g.Items)+1); err != nil {
		return err
	}
	return f.SaveAs(path)
}

// writeReferenceFile: 모든 묶음을 업로드 양식 그대로 한 시트에 모은 확인용 파일
// (A~O 업로드 양식, P 빈칸, Q 단위, R 구분)
func writeReferenceFile(path string, groups []OrderGroup, fileNames []string, vendors map[string]string) error {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "전체"
	f.SetSheetName("Sheet1", sheet)

	if err := f.SetSheetRow(sheet, "A1", &referenceHeaders); err != nil {
		return err
	}

	r := 2
	for gi, g := range groups {
		for _, it := range g.Items {
			row, err := uploadRow(it, vendors[g.Contract])
			if err != nil {
				return err
			}
			row = append(row, nil, it.Unit, fileNames[gi]) // P 진짜 빈칸, Q 단위, R 구분
			cellRef, _ := excelize.CoordinatesToCellName(1, r)
			if err := f.SetSheetRow(sheet, cellRef, &row); err != nil {
				return err
			}
			r++
		}
	}

	if err := applyUploadLayout(f, sheet, r-1); err != nil {
		return err
	}
	f.SetColWidth(sheet, "R", "R", 28)
	f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}) // 헤더 고정
	f.AutoFilter(sheet, fmt.Sprintf("A1:R%d", r-1), nil)                                                     // 필터 버튼

	return f.SaveAs(path)
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xuri/excelize/v2"
)

// 제목 앞부분 "10월 구매현황[10. 1. ~ 10. 31" 을 찾는 정규식 (뒷부분 ", 일자별, ..."은 그대로 둠)
var settlementTitleRe = regexp.MustCompile(`^\d+월 구매현황\[\d+\. 1\.? ~ \d+\. \d+`)

// 파일 이름 앞부분 "26. 10월" 을 찾는 정규식
var settlementFileRe = regexp.MustCompile(`^\d+\. \d+월`)

// findSettlementTemplate: 양식 폴더에서 "(계약).xlsx"로 끝나는 파일을 찾음
func findSettlementTemplate(dir, sheetName string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	suffix := "(" + sheetName + ").xlsx"
	for _, e := range entries {
		// "~$"로 시작하는 건 엑셀이 열려 있을 때 생기는 임시 파일
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) && !strings.HasPrefix(e.Name(), "~$") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("양식 폴더에 '*%s' 파일이 없습니다", suffix)
}

// settlementLayout: 양식에서 찾아낸 행 위치
type settlementLayout struct {
	dataStart   int // 첫 데이터 행 (A열이 1)
	totalRow    int // 합계 행
	lastSummary int // R~U열 월 종합 표의 마지막 행
}

func findSettlementLayout(f *excelize.File, sheet string) (settlementLayout, error) {
	var l settlementLayout
	rows, err := f.GetRows(sheet)
	if err != nil {
		return l, err
	}
	header := 0
	for i, row := range rows {
		a := ""
		if len(row) > 0 {
			a = strings.ReplaceAll(row[0], " ", "") // "합        계" → "합계"
		}
		switch {
		case header == 0 && a == "순번":
			header = i + 1
		case header > 0 && l.dataStart == 0 && a == "1":
			l.dataStart = i + 1
		case l.dataStart > 0 && a == "합계":
			l.totalRow = i + 1
		}
		if l.totalRow > 0 {
			break
		}
		// R~U열(18~21번째 칸)에 값이 있으면 월 종합 표가 있는 행
		for c := 17; c < len(row) && c <= 20; c++ {
			if strings.TrimSpace(row[c]) != "" {
				l.lastSummary = i + 1
			}
		}
	}
	if header == 0 || l.dataStart == 0 || l.totalRow == 0 {
		return l, fmt.Errorf("양식에서 순번 헤더/첫 데이터 행/합계 행을 찾지 못했습니다")
	}
	return l, nil
}

// fillSettlementTemplate: 지난달 결산 파일을 양식으로 써서 이번 달 데이터로 채운 새 파일을 만든다
func fillSettlementTemplate(templatePath, outPath string, s SettlementSheet, month, lastDay int) error {
	f, err := openXLSXNormalized(templatePath)
	if err != nil {
		return err
	}
	defer f.Close()
	sheet := f.GetSheetName(0)

	l, err := findSettlementLayout(f, sheet)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(templatePath), err)
	}

	// 데이터 칸 = 첫 데이터 행 ~ 합계 바로 위 예비 행 앞까지
	area := l.totalRow - 1 - l.dataStart
	// 필요한 칸 = 이번 달 건수. 단, 월 종합 표가 있는 행까지는 지우면 안 됨
	want := len(s.Rows)
	if minRows := l.lastSummary - l.dataStart + 1; want < minRows {
		want = minRows
	}

	// 모자라면: 마지막 데이터 행을 서식·수식째 복사해서 끼워 넣음
	src := l.totalRow - 2
	for area < want {
		if src <= l.lastSummary {
			return fmt.Errorf("양식의 데이터 행이 너무 적어서 복사할 견본 행이 없습니다")
		}
		if err := f.DuplicateRowTo(sheet, src, src+1); err != nil {
			return err
		}
		area++
	}
	// 남으면: 아래쪽 데이터 행부터 삭제
	for area > want {
		if err := f.RemoveRow(sheet, l.dataStart+area-1); err != nil {
			return err
		}
		area--
	}

	// A~J열 채우기 (서식은 양식 것을 그대로 쓰고 값만 바꿈)
	for i := 0; i < area; i++ {
		values := make([]interface{}, 10) // 데이터가 없는 행은 전부 nil = 빈 칸
		if i < len(s.Rows) {
			r := s.Rows[i]
			values = []interface{}{
				i + 1, r.Date, r.Place, textOrNil(r.Name), textOrNil(r.Spec),
				r.Tax, textOrNil(r.Unit), r.Qty, r.Price, r.Amount,
			}
		}
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, l.dataStart+i)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
	}

	// 필터 범위: 행을 끼워 넣을 때는 자동으로 안 늘어나서 직접 지정
	lastData := l.dataStart + area - 1
	if err := f.AutoFilter(sheet, fmt.Sprintf("A%d:U%d", l.dataStart-1, lastData), nil); err != nil {
		return err
	}

	// 제목의 월 바꾸기: "10월 구매현황[10. 1. ~ 10. 31" → "11월 구매현황[11. 1. ~ 11. 30"
	title, _ := f.GetCellValue(sheet, "A1")
	title = settlementTitleRe.ReplaceAllString(title, fmt.Sprintf("%d월 구매현황[%d. 1. ~ %d. %d", month, month, month, lastDay))
	f.SetCellValue(sheet, "A1", title)

	// 파일을 열 때 수식(합계, 월 종합)을 다시 계산하도록 설정
	recalc := true
	if err := f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: &recalc}); err != nil {
		return err
	}
	return f.SaveAs(outPath)
}

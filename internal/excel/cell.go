package excel

import (
	"strconv"
	"strings"
)

// Cell: 행의 i번째 칸 값. 행이 짧아서 칸이 없으면 "" 반환
func Cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// ParseNumber: "1,145,300" 같은 문자열을 숫자로. 빈 칸은 0
func ParseNumber(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

// TextOrNil: 빈 글자는 진짜 빈 칸(nil)으로 → 붙여넣었을 때 양식 칸이 비어 있게
func TextOrNil(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/korean"
)

// PurchaseRow: ESP 기간별구매현황[일자별]의 품목 한 줄
type PurchaseRow struct {
	Date      string  // 입고일자 (2026-10-01)
	Code      string  // 품번
	Name      string  // 품명
	Spec      string  // 규격
	Qty       float64 // 수량
	Unit      string  // 단위
	Price     float64 // 단가
	Amount    float64 // 금액 (공급가액)
	VAT       float64 // 부가세
	Total     float64 // 합계금액
	Vendor    string  // 거래처명
	Site      string  // 현장명
	Warehouse string  // 입고창고
}

var espRequiredCols = []string{
	"입고일자", "품번", "품명", "규격", "수량", "단위", "단가",
	"금액", "부가세", "합계금액", "거래처명", "현장명", "입고창고",
}

// readHTMLTable: EUC-KR로 저장된 HTML 파일에서 표의 글자만 꺼내 [행][칸] 목록으로 만든다
func readHTMLTable(path string) ([][]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// EUC-KR → UTF-8 (Go 문자열은 UTF-8)
	text, err := korean.EUCKR.NewDecoder().Bytes(raw)
	if err != nil {
		return nil, fmt.Errorf("한글 인코딩 변환 실패: %w", err)
	}

	doc, err := html.Parse(bytes.NewReader(text))
	if err != nil {
		return nil, fmt.Errorf("HTML 읽기 실패: %w", err)
	}

	var rows [][]string
	// walk: HTML 요소를 위에서 아래로 훑으면서 <tr>을 찾는 함수 (자기 자신을 다시 부름)
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var row []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					row = append(row, strings.TrimSpace(textOf(c)))
				}
			}
			rows = append(rows, row)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return rows, nil
}

// textOf: 요소 안의 글자를 모두 이어 붙임
func textOf(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textOf(c))
	}
	return sb.String()
}

// readESP: ESP 기간별구매현황 파일을 읽어서 품목 목록으로 만든다
func readESP(path string) ([]PurchaseRow, error) {
	rows, err := readHTMLTable(path)
	if err != nil {
		return nil, err
	}

	// 필요한 열 이름이 모두 있는 행을 헤더로
	headerRow := -1
	var col map[string]int
	for r, row := range rows {
		m := map[string]int{}
		for c, v := range row {
			m[v] = c
		}
		found := true
		for _, name := range espRequiredCols {
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
		return nil, fmt.Errorf("헤더 행을 찾을 수 없습니다. ESP 기간별구매현황 파일이 맞나요?")
	}

	var result []PurchaseRow
	for r := headerRow + 1; r < len(rows); r++ {
		row := rows[r]

		// 품번이 없으면 [ 합 계 ] 줄 → 건너뜀
		code := cell(row, col["품번"])
		if code == "" {
			continue
		}

		p := PurchaseRow{
			Date:      cell(row, col["입고일자"]),
			Code:      code,
			Name:      cell(row, col["품명"]),
			Spec:      cell(row, col["규격"]),
			Unit:      cell(row, col["단위"]),
			Vendor:    cell(row, col["거래처명"]),
			Site:      cell(row, col["현장명"]),
			Warehouse: cell(row, col["입고창고"]),
		}

		nums := []struct {
			name string
			dst  *float64
		}{
			{"수량", &p.Qty},
			{"단가", &p.Price},
			{"금액", &p.Amount},
			{"부가세", &p.VAT},
			{"합계금액", &p.Total},
		}
		for _, n := range nums {
			v, err := parseNumber(cell(row, col[n.name]))
			if err != nil {
				return nil, fmt.Errorf("%d행 %s 값이 숫자가 아닙니다: %q", r+1, n.name, cell(row, col[n.name]))
			}
			*n.dst = v
		}

		result = append(result, p)
	}
	return result, nil
}

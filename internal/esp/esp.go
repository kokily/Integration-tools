package esp

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/korean"

	"integration-tools/internal/excel"
)

// PurchaseRow: ESP 기간별구매현황[일자별]의 품목 한 줄
type PurchaseRow struct {
	Seq       string  // 순번 (원본 그대로, 쉼표 제거)
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
	"순번", "입고일자", "품번", "품명", "규격", "수량", "단위", "단가",
	"금액", "부가세", "합계금액", "거래처명", "현장명", "입고창고",
}

// ReadTable: EUC-KR로 저장된 HTML 파일에서 표의 글자만 꺼내 [행][칸] 목록으로 만든다
func ReadTable(path string) ([][]string, error) {
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
					row = append(row, cellText(c))
					// colspan="2"처럼 여러 칸을 차지하는 셀은 남는 칸을 빈 값으로 채워 열 위치를 맞춤
					for i := 1; i < colspan(c); i++ {
						row = append(row, "")
					}
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

// lineBreakRemover: 셀 글자 안의 줄바꿈 제거 (예전에 엑셀로 다시 저장할 때도 없어졌음)
var lineBreakRemover = strings.NewReplacer("\r", "", "\n", "")

// cellText: <td> 안의 글자에서 줄바꿈을 지우고 앞뒤 공백을 정리
func cellText(n *html.Node) string {
	return strings.TrimSpace(lineBreakRemover.Replace(textOf(n)))
}

// colspan: <td colspan="N">의 N (없거나 잘못된 값이면 1)
func colspan(n *html.Node) int {
	for _, a := range n.Attr {
		if a.Key == "colspan" {
			if v, err := strconv.Atoi(a.Val); err == nil && v > 1 {
				return v
			}
		}
	}
	return 1
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

// Read: ESP 기간별구매현황 파일을 읽어서 품목 목록으로 만든다
func Read(path string) ([]PurchaseRow, error) {
	rows, err := ReadTable(path)
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
		code := excel.Cell(row, col["품번"])
		if code == "" {
			continue
		}

		p := PurchaseRow{
			Seq:       strings.ReplaceAll(excel.Cell(row, col["순번"]), ",", ""),
			Date:      excel.Cell(row, col["입고일자"]),
			Code:      code,
			Name:      excel.Cell(row, col["품명"]),
			Spec:      excel.Cell(row, col["규격"]),
			Unit:      excel.Cell(row, col["단위"]),
			Vendor:    excel.Cell(row, col["거래처명"]),
			Site:      excel.Cell(row, col["현장명"]),
			Warehouse: excel.Cell(row, col["입고창고"]),
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
			v, err := excel.ParseNumber(excel.Cell(row, col[n.name]))
			if err != nil {
				return nil, fmt.Errorf("%d행 %s 값이 숫자가 아닙니다: %q", r+1, n.name, excel.Cell(row, col[n.name]))
			}
			*n.dst = v
		}

		result = append(result, p)
	}
	return result, nil
}

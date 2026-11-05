package main

import (
	"fmt"
	"strings"
	"time"
)

// SettlementRow: 결산서 양식 B~J열 한 줄 (A열 순번은 쓸 때 매김)
type SettlementRow struct {
	Date   string  // B 구매일자 (10월 1일)
	Place  string  // C 납품장소
	Name   string  // D 품명
	Spec   string  // E 규격
	Tax    string  // F 세 (면/과)
	Unit   string  // G 단위
	Qty    float64 // H 수량
	Price  float64 // I 단가
	Amount float64 // J 금액 (면: 금액, 과: 합계금액)
}

// SettlementSheet: 결과 파일 하나 분량 (계약, 비계약, 직원계약, 직원비계약)
type SettlementSheet struct {
	Name string
	Rows []SettlementRow
}

// toSettlementRow: ESP 한 줄 → 결산서 한 줄
func toSettlementRow(p PurchaseRow, cfg SettlementConfig) (SettlementRow, error) {
	d, err := time.Parse("2006-01-02", p.Date)
	if err != nil {
		return SettlementRow{}, fmt.Errorf("입고일자 형식 오류: %q (%s)", p.Date, p.Name)
	}

	// 납품장소: 표에 있으면 바꾸고, 없으면 "-비계약"만 떼기
	place := p.Site
	if mapped, ok := cfg.PlaceMap[place]; ok {
		place = mapped
	} else {
		place = strings.TrimSuffix(place, cfg.RemoveSuffix)
	}

	// 면세는 금액, 과세는 부가세 포함 합계금액
	tax, amount := "면", p.Amount
	if p.VAT != 0 {
		tax, amount = "과", p.Total
	}

	return SettlementRow{
		Date:   fmt.Sprintf("%d월 %d일", d.Month(), d.Day()),
		Place:  place,
		Name:   p.Name,
		Spec:   p.Spec,
		Tax:    tax,
		Unit:   p.Unit,
		Qty:    p.Qty,
		Price:  p.Price,
		Amount: amount,
	}, nil
}

// buildSettlement: 계약/비계약 원본을 결과 파일 4개 분량으로 나눔 (원본 순서 유지)
func buildSettlement(contract, nonContract []PurchaseRow, cfg SettlementConfig) ([]SettlementSheet, error) {
	sheets := []SettlementSheet{
		{Name: "계약"}, {Name: "비계약"}, {Name: "직원계약"}, {Name: "직원비계약"},
	}

	// add: rows를 변환해서 normal 또는 employee 번호의 시트에 넣음
	add := func(rows []PurchaseRow, normal, employee int) error {
		for _, p := range rows {
			r, err := toSettlementRow(p, cfg)
			if err != nil {
				return err
			}
			idx := normal
			if strings.Contains(p.Site, cfg.EmployeeSite) {
				idx = employee
			}
			sheets[idx].Rows = append(sheets[idx].Rows, r)
		}
		return nil
	}

	if err := add(contract, 0, 2); err != nil {
		return nil, err
	}
	if err := add(nonContract, 1, 3); err != nil {
		return nil, err
	}
	return sheets, nil
}

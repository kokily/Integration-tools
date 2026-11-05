package report

import "fmt"

// Report: 기능을 실행한 결과. 터미널(main.go)과 화면(web.go)이 같이 쓴다
type Report struct {
	Lines    []string // 진행 내용 (예: "저장: 01_양식뷔페-계약-면.xlsx (31건)")
	Warnings []string // 확인이 필요한 내용 (예: "설정에 없는 사업장: ○○○")
	OutDir   string   // 결과가 저장된 폴더
	Files    []string // 만든 파일 (전체 경로)
	Fixes    []Fix    // 경고 중 설정 화면에서 바로 추가할 수 있는 항목
}

// Fix: 설정에 없어서 경고가 난 항목. 화면의 [설정에 추가] 버튼에 쓴다
type Fix struct {
	Kind  string `json:"kind"`  // site(발주서 사업장), contract(발주서 계약구분), warehouse(소모품 창고), rate(소모품 낙찰률 연도)
	Value string `json:"value"` // 추가할 이름 (사업장 이름, 창고 이름, 연도 2자리)
}

// AddFix: 같은 항목은 한 번만 넣음
func (r *Report) AddFix(kind, value string) {
	for _, f := range r.Fixes {
		if f.Kind == kind && f.Value == value {
			return
		}
	}
	r.Fixes = append(r.Fixes, Fix{Kind: kind, Value: value})
}

// Linef: 진행 내용 한 줄 추가 (fmt.Printf와 같은 모양으로 씀)
func (r *Report) Linef(format string, args ...interface{}) {
	r.Lines = append(r.Lines, fmt.Sprintf(format, args...))
}

// Warnf: 경고 한 줄 추가
func (r *Report) Warnf(format string, args ...interface{}) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

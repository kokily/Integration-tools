package config

import (
	"fmt"
	"regexp"
	"strings"
)

var yearRe = regexp.MustCompile(`^\d{2}$`)

// Validate: 저장하기 전에 설정을 검사. 문제가 있으면 화면에 보여 줄 문구 목록을 돌려준다
func Validate(cfg *Config) []string {
	var errs []string
	add := func(format string, args ...interface{}) { errs = append(errs, fmt.Sprintf(format, args...)) }

	// checkList: 목록에 빈 이름이나 같은 이름이 두 번 있는지
	checkList := func(label string, items []string) {
		seen := map[string]bool{}
		for i, v := range items {
			v = strings.TrimSpace(v)
			if v == "" {
				add("%s: %d번째 칸이 비어 있습니다", label, i+1)
				continue
			}
			if seen[v] {
				add("%s: '%s' 이름이 두 번 들어 있습니다", label, v)
			}
			seen[v] = true
		}
	}

	// 발주서
	o := cfg.WelstoryOrder
	checkList("발주서 사업장 순서", o.Sites)
	checkList("발주서 계약구분 순서", o.Contracts)
	for _, c := range o.Contracts {
		if strings.TrimSpace(o.VendorName[c]) == "" {
			add("발주서 거래처명: 계약구분 '%s'의 거래처명이 없습니다", c)
		}
	}

	// 결산서
	s := cfg.Settlement
	if strings.TrimSpace(s.EmployeeSite) == "" {
		add("결산서: 직원식당 구분 글자가 비어 있습니다")
	}
	for k, v := range s.PlaceMap {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			add("결산서 납품장소 바꾸기: 빈 칸이 있습니다 ('%s' → '%s')", k, v)
		}
	}

	// 소모품
	e := cfg.Expendables
	for year, rate := range e.Rates {
		if !yearRe.MatchString(year) {
			add("소모품 낙찰률: 연도 '%s' → 숫자 2자리(예: 26)로 적어 주세요", year)
		}
		if rate <= 0 || rate > 1 {
			add("소모품 낙찰률: %s년 값 %.5g%%가 0~100%% 범위를 벗어났습니다", year, rate*100)
		}
	}
	owner := map[string]string{} // 원본 창고 → 들어 있는 시트
	for sheet, sources := range e.Warehouses {
		if strings.TrimSpace(sheet) == "" {
			add("소모품 창고 매핑: 이름이 빈 시트가 있습니다")
		}
		checkList("소모품 창고 매핑 ["+sheet+"]", sources)
		for _, src := range sources {
			if prev, ok := owner[src]; ok && prev != sheet {
				add("소모품 창고 매핑: 창고 '%s' → '%s', '%s' 두 시트에 모두 들어 있습니다", src, prev, sheet)
			}
			owner[src] = sheet
		}
	}
	checkList("소모품 제외 창고", e.Exclude)
	for _, x := range e.Exclude {
		if sheet, ok := owner[x]; ok {
			add("소모품: 창고 '%s' → 제외 창고와 '%s' 시트에 모두 들어 있습니다", x, sheet)
		}
	}

	// 거래처별
	checkList("거래처 시트 순서", cfg.Vendors.Order)
	checkList("시트를 만들지 않을 거래처", cfg.Vendors.Exclude)

	return errs
}

package main

import (
	"fmt"
	"sort"
	"time"
)

// 면/과 순서는 항상 고정
var taxOrder = []string{"면", "과"}

// OrderGroup: 업로드 파일 하나에 들어갈 품목 묶음
type OrderGroup struct {
	Site     string
	Contract string
	Tax      string
	Items    []OrderItem
}

// Name: "양식뷔페-계약-면" (파일 이름으로 쓸 값)
func (g OrderGroup) Name() string {
	return g.Site + "-" + g.Contract + "-" + g.Tax
}

func taxType(it OrderItem) string {
	if it.VAT == 0 {
		return "면"
	}
	return "과"
}

// indexOf: 목록에서 위치. 목록에 없으면 맨 뒤(len)로 취급
func indexOf(list []string, v string) int {
	for i, s := range list {
		if s == v {
			return i
		}
	}
	return len(list)
}

// groupOrder: 품목을 사업장-계약구분-면과로 묶고 설정 순서대로 정렬.
// 설정에 없는 사업장/계약구분은 경고 목록으로 돌려줌
func groupOrder(items []OrderItem, cfg WelstoryOrderConfig) ([]OrderGroup, []string) {
	byName := map[string]*OrderGroup{}
	for _, it := range items {
		g := OrderGroup{Site: it.Site, Contract: it.Contract, Tax: taxType(it)}
		if byName[g.Name()] == nil {
			byName[g.Name()] = &g
		}
		byName[g.Name()].Items = append(byName[g.Name()].Items, it)
	}

	var groups []OrderGroup
	for _, g := range byName {
		groups = append(groups, *g)
	}

	// map은 꺼낼 때마다 순서가 뒤섞이므로 반드시 정렬해야 함
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if x, y := indexOf(cfg.Sites, a.Site), indexOf(cfg.Sites, b.Site); x != y {
			return x < y
		}
		if a.Site != b.Site {
			return a.Site < b.Site // 설정에 없는 사업장끼리는 가나다순
		}
		if x, y := indexOf(cfg.Contracts, a.Contract), indexOf(cfg.Contracts, b.Contract); x != y {
			return x < y
		}
		if a.Contract != b.Contract {
			return a.Contract < b.Contract
		}
		return indexOf(taxOrder, a.Tax) < indexOf(taxOrder, b.Tax)
	})

	var warnings []string
	seen := map[string]bool{}
	for _, g := range groups {
		if indexOf(cfg.Sites, g.Site) == len(cfg.Sites) && !seen["사업장:"+g.Site] {
			seen["사업장:"+g.Site] = true
			warnings = append(warnings, fmt.Sprintf("설정에 없는 사업장: %s (맨 뒤에 배치)", g.Site))
		}
		if indexOf(cfg.Contracts, g.Contract) == len(cfg.Contracts) && !seen["계약:"+g.Contract] {
			seen["계약:"+g.Contract] = true
			warnings = append(warnings, fmt.Sprintf("설정에 없는 계약구분: %s", g.Contract))
		}
	}
	return groups, warnings
}

// codeWithYear: 주문 일자의 연도 2자리 + 품목코드 ("2026/10/29", "1000646505" → "261000646505")
func codeWithYear(date, code string) (string, error) {
	t, err := time.Parse("2006/01/02", date)
	if err != nil {
		return "", fmt.Errorf("일자 형식 오류: %q", date)
	}
	return t.Format("06") + code, nil
}

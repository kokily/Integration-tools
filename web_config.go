package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// registerConfigHandlers: /api/config  GET = 지금 설정 읽기, POST = 검사 후 저장 (이전 설정은 백업)
func registerConfigHandlers(mux *http.ServeMux, dir string) {
	path := filepath.Join(dir, "config.json")
	backupDir := filepath.Join(dir, "설정백업")

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg, err := loadConfig(path)
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, cfg)

		case http.MethodPost:
			var cfg Config
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields() // 모르는 항목이 섞여 오면 거절
			if err := dec.Decode(&cfg); err != nil {
				writeJSON(w, map[string]interface{}{"ok": false, "errors": []string{"설정 형식 오류: " + err.Error()}})
				return
			}
			if errs := validateConfig(&cfg); len(errs) > 0 {
				writeJSON(w, map[string]interface{}{"ok": false, "errors": errs})
				return
			}
			if err := saveConfig(path, backupDir, &cfg); err != nil {
				writeJSON(w, map[string]interface{}{"ok": false, "errors": []string{"저장 실패: " + err.Error()}})
				return
			}
			writeJSON(w, map[string]interface{}{"ok": true})

		default:
			http.Error(w, "GET, POST만 받습니다", http.StatusMethodNotAllowed)
		}
	})
}

// saveConfig: 지금 설정 파일을 백업 폴더에 복사해 두고, 새 설정을 저장한다
func saveConfig(path, backupDir string, cfg *Config) error {
	if _, err := os.Stat(path); err == nil {
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return err
		}
		backup := filepath.Join(backupDir, "config_"+time.Now().Format("20060102_150405")+".json")
		if err := copyFile(path, backup); err != nil {
			return fmt.Errorf("백업 실패: %w", err)
		}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // "<", ">", "&"를 \u003c 같은 모양으로 바꾸지 않음
	enc.SetIndent("", "  ")  // 사람이 읽기 좋게 들여쓰기
	if err := enc.Encode(cfg); err != nil {
		return err
	}

	// 임시 파일에 다 쓴 뒤 바꿔 끼움 (쓰는 도중에 꺼져도 설정 파일이 깨지지 않게)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var yearRe = regexp.MustCompile(`^\d{2}$`)

// validateConfig: 저장하기 전에 설정을 검사. 문제가 있으면 화면에 보여 줄 문구 목록을 돌려준다
func validateConfig(cfg *Config) []string {
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

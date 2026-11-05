package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 한 번에 올릴 수 있는 파일 크기 합계 (현재고현황이 16MB 정도라 넉넉하게)
const maxUploadBytes = 200 << 20 // 200MB

// jobResponse: 화면으로 돌려보내는 실행 결과
type jobResponse struct {
	OK       bool     `json:"ok"`
	Error    string   `json:"error,omitempty"`
	Lines    []string `json:"lines"`
	Warnings []string `json:"warnings"`
	OutDir   string   `json:"outDir"`
	Files    []string `json:"files"` // 만든 파일 이름
}

// uploads: 입력 칸 이름 → 그 칸으로 올라온 파일들의 임시 저장 경로 (여러 개 올릴 수 있는 칸도 있음)
type uploads map[string][]string

// first: 입력 칸의 첫 번째 파일 경로 (없으면 "")
func (u uploads) first(field string) string {
	if len(u[field]) == 0 {
		return ""
	}
	return u[field][0]
}

// webJob: 화면에서 실행할 수 있는 기능 하나
type webJob struct {
	folder   string            // 결과 폴더 안의 하위 폴더 이름
	required map[string]string // 꼭 올려야 하는 입력 칸 이름 → 화면에 보이는 이름
	run      func(files uploads, outRoot string, cfg *Config) (*Report, error)
}

// registerJobHandlers: /api/run/○○ (기능 실행), /api/templates (보관된 양식), /api/open (결과 폴더 열기)
func registerJobHandlers(mux *http.ServeMux, dir string) {
	resultRoot := filepath.Join(dir, "결과")
	settlementTpl := templateDir(dir, "결산서")
	expendablesTpl := templateDir(dir, "소모품")

	jobs := map[string]webJob{
		"order": {
			folder:   "발주서",
			required: map[string]string{"src": "웰스토리 원본 파일"},
			run: func(f uploads, out string, cfg *Config) (*Report, error) {
				return RunOrder(f.first("src"), out, cfg)
			},
		},
		"settlement": {
			folder:   "결산서",
			required: map[string]string{"contract": "계약 원본", "noncontract": "비계약 원본"},
			run: func(f uploads, out string, cfg *Config) (*Report, error) {
				return runSettlementWithTemplates(f, out, settlementTpl, cfg)
			},
		},
		"expendables": {
			folder:   "소모품",
			required: map[string]string{"src": "현재고현황(창고별) 원본"},
			run: func(f uploads, out string, cfg *Config) (*Report, error) {
				return runExpendablesWithTemplate(f, out, expendablesTpl, cfg)
			},
		},
		"vendors": {
			folder:   "거래처별",
			required: map[string]string{"src": "기간별구매현황 원본"},
			run: func(f uploads, out string, cfg *Config) (*Report, error) {
				return RunVendors(f.first("src"), out, cfg)
			},
		},
	}

	for name, job := range jobs {
		mux.HandleFunc("/api/run/"+name, jobHandler(dir, filepath.Join(resultRoot, job.folder), job))
	}

	mux.HandleFunc("/api/templates", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]templateInfo{
			"settlement":  readTemplateInfo(settlementTpl),
			"expendables": readTemplateInfo(expendablesTpl),
		})
	})
	mux.HandleFunc("/api/open", openFolderHandler(resultRoot))
}

// runSettlementWithTemplates: 결산서 실행. 양식을 올렸으면 보관 양식을 그것으로 바꾸고,
// 실행 뒤에는 이번 결과 4개를 다음 달 양식으로 보관한다
func runSettlementWithTemplates(f uploads, out, tplDir string, cfg *Config) (*Report, error) {
	if len(f["templates"]) > 0 {
		if err := replaceTemplates(tplDir, f["templates"]); err != nil {
			return nil, fmt.Errorf("양식 보관 실패: %w", err)
		}
	}

	useTpl := ""
	if len(listTemplates(tplDir)) > 0 {
		useTpl = tplDir
	}

	rep, err := RunSettlement(f.first("contract"), f.first("noncontract"), useTpl, out, cfg)
	if err != nil {
		return nil, err
	}

	if useTpl == "" {
		rep.Warnf("보관된 결산자료 양식이 없어 붙여넣기용 파일만 만들었습니다. 지난달 결산자료 4개를 '양식' 칸에 같이 올리면 다음부터 자동으로 씁니다.")
		return rep, nil
	}

	var made []string // 붙여넣기용 파일을 뺀 결산자료 4개
	for _, file := range rep.Files {
		if filepath.Base(file) != "결산서_붙여넣기.xlsx" {
			made = append(made, file)
		}
	}
	if err := replaceTemplates(tplDir, made); err != nil {
		rep.Warnf("다음 달 양식으로 보관하지 못했습니다: %v", err)
	} else {
		rep.Linef("다음 달 양식으로 보관: 결산자료 %d개", len(made))
	}
	return rep, nil
}

// runExpendablesWithTemplate: 소모품 결산 실행. 양식을 올렸으면 보관 양식을 그것으로 바꾸고,
// 실행 뒤에는 이번 결과를 다음 양식으로 보관한다
func runExpendablesWithTemplate(f uploads, out, tplDir string, cfg *Config) (*Report, error) {
	if tpl := f.first("template"); tpl != "" {
		if err := replaceTemplates(tplDir, []string{tpl}); err != nil {
			return nil, fmt.Errorf("양식 보관 실패: %w", err)
		}
	}

	stored := listTemplates(tplDir)
	if len(stored) == 0 {
		return nil, errors.New("보관된 소모품 결산 양식이 없습니다. 처음 한 번은 지난 소모품 결산 파일을 '양식' 칸에 같이 올려 주세요")
	}
	if len(stored) > 1 {
		return nil, fmt.Errorf("양식 폴더(%s)에 xlsx 파일이 %d개 있습니다. 하나만 남겨 주세요", tplDir, len(stored))
	}

	rep, err := RunExpendables(f.first("src"), filepath.Join(tplDir, stored[0]), out, cfg)
	if err != nil {
		return nil, err
	}

	if err := replaceTemplates(tplDir, rep.Files); err != nil {
		rep.Warnf("다음 양식으로 보관하지 못했습니다: %v", err)
	} else {
		rep.Linef("다음 양식으로 보관: %s", filepath.Base(rep.Files[0]))
	}
	return rep, nil
}

// jobHandler: 파일을 받아 임시 폴더에 저장 → 기능 실행 → 결과를 JSON으로 응답
func jobHandler(dir, outRoot string, job webJob) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST만 받습니다", http.StatusMethodNotAllowed)
			return
		}
		fail := func(format string, args ...interface{}) {
			writeJSON(w, jobResponse{OK: false, Error: fmt.Sprintf(format, args...)})
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			fail("파일을 받지 못했습니다: %v", err)
			return
		}

		// 올라온 파일은 임시 폴더에 원래 이름 그대로 저장하고, 끝나면 폴더째 지움
		tmp, err := os.MkdirTemp("", "integration-tools-*")
		if err != nil {
			fail("임시 폴더를 만들지 못했습니다: %v", err)
			return
		}
		defer os.RemoveAll(tmp)

		files, err := saveUploads(r.MultipartForm, tmp)
		if err != nil {
			fail("%v", err)
			return
		}
		for field, label := range job.required {
			if len(files[field]) == 0 {
				fail("%s 파일을 올려 주세요", label)
				return
			}
		}

		// 설정은 실행할 때마다 새로 읽음 (설정 화면에서 바꾼 내용이 바로 반영되도록)
		cfg, err := loadConfig(filepath.Join(dir, "config.json"))
		if err != nil {
			fail("%v", err)
			return
		}

		rep, err := job.run(files, outRoot, cfg)
		if err != nil {
			fail("%v", err)
			return
		}

		resp := jobResponse{OK: true, Lines: rep.Lines, Warnings: rep.Warnings, OutDir: rep.OutDir}
		for _, f := range rep.Files {
			resp.Files = append(resp.Files, filepath.Base(f))
		}
		writeJSON(w, resp)
	}
}

// saveUploads: 올라온 파일을 모두 "임시폴더\입력칸이름\원래파일이름"으로 저장
func saveUploads(form *multipart.Form, tmp string) (uploads, error) {
	files := uploads{}
	for field, headers := range form.File {
		for _, h := range headers {
			if h.Filename == "" { // 비워 둔 선택 입력 칸
				continue
			}
			path, err := saveFileHeader(h, filepath.Join(tmp, field))
			if err != nil {
				return nil, err
			}
			files[field] = append(files[field], path)
		}
	}
	return files, nil
}

// saveFileHeader: 올라온 파일 하나를 dir 폴더에 원래 이름으로 저장
func saveFileHeader(h *multipart.FileHeader, dir string) (string, error) {
	src, err := h.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(h.Filename)) // 파일 이름만 사용 (경로가 섞여 와도 무시)
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return "", fmt.Errorf("파일 저장 실패: %w", err)
	}
	return path, nil
}

// openFolderHandler: 탐색기로 결과 폴더를 연다. 결과 폴더(resultRoot) 안쪽만 열 수 있음
func openFolderHandler(resultRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST만 받습니다", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]string{"error": "요청 형식 오류"})
			return
		}

		// 결과 폴더 밖(..)으로 나가는 경로는 거절
		rel, err := filepath.Rel(resultRoot, filepath.Clean(req.Path))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			writeJSON(w, map[string]string{"error": "결과 폴더 밖은 열 수 없습니다"})
			return
		}
		if info, err := os.Stat(req.Path); err != nil || !info.IsDir() {
			writeJSON(w, map[string]string{"error": "폴더가 없습니다"})
			return
		}

		if err := exec.Command("explorer", filepath.Clean(req.Path)).Start(); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]string{"error": ""})
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
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

// jobFunc: 올라온 파일들(입력 칸 이름 → 임시 저장 경로)과 결과 폴더를 받아 기능을 실행
type jobFunc func(files map[string]string, outRoot string, cfg *Config) (*Report, error)

// webJob: 화면에서 실행할 수 있는 기능 하나
type webJob struct {
	folder string   // 결과 폴더 안의 하위 폴더 이름
	fields []string // 화면에서 올려야 하는 파일 입력 칸 이름
	run    jobFunc
}

// registerJobHandlers: /api/run/○○ (기능 실행) 와 /api/open (결과 폴더 열기) 를 등록
func registerJobHandlers(mux *http.ServeMux, dir string) {
	resultRoot := filepath.Join(dir, "결과")

	jobs := map[string]webJob{
		"order": {
			folder: "발주서",
			fields: []string{"src"},
			run: func(f map[string]string, out string, cfg *Config) (*Report, error) {
				return RunOrder(f["src"], out, cfg)
			},
		},
	}

	for name, job := range jobs {
		mux.HandleFunc("/api/run/"+name, jobHandler(dir, filepath.Join(resultRoot, job.folder), job))
	}
	mux.HandleFunc("/api/open", openFolderHandler(resultRoot))
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

		paths := map[string]string{}
		for _, field := range job.fields {
			path, err := saveUpload(r, field, filepath.Join(tmp, field))
			if err != nil {
				fail("%v", err)
				return
			}
			paths[field] = path
		}

		// 설정은 실행할 때마다 새로 읽음 (설정 화면에서 바꾼 내용이 바로 반영되도록)
		cfg, err := loadConfig(filepath.Join(dir, "config.json"))
		if err != nil {
			fail("%v", err)
			return
		}

		rep, err := job.run(paths, outRoot, cfg)
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

// saveUpload: 입력 칸(field)으로 올라온 파일을 dir 폴더에 원래 파일 이름으로 저장하고 경로를 돌려줌
func saveUpload(r *http.Request, field, dir string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", fmt.Errorf("올린 파일이 없습니다 (%s)", field)
	}
	defer file.Close()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(header.Filename)) // 파일 이름만 사용 (경로가 섞여 와도 무시)
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
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

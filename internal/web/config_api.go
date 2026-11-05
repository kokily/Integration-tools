package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"integration-tools/internal/config"
)

// registerConfigHandlers: /api/config  GET = 지금 설정 읽기, POST = 검사 후 저장 (이전 설정은 백업)
func registerConfigHandlers(mux *http.ServeMux, dir string) {
	path := filepath.Join(dir, "config.json")
	backupDir := filepath.Join(dir, "설정백업")

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg, err := config.Load(path)
			if err != nil {
				writeJSON(w, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, cfg)

		case http.MethodPost:
			var cfg config.Config
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields() // 모르는 항목이 섞여 오면 거절
			if err := dec.Decode(&cfg); err != nil {
				writeJSON(w, map[string]interface{}{"ok": false, "errors": []string{"설정 형식 오류: " + err.Error()}})
				return
			}
			if errs := config.Validate(&cfg); len(errs) > 0 {
				writeJSON(w, map[string]interface{}{"ok": false, "errors": errs})
				return
			}
			if err := config.Save(path, backupDir, &cfg); err != nil {
				writeJSON(w, map[string]interface{}{"ok": false, "errors": []string{"저장 실패: " + err.Error()}})
				return
			}
			writeJSON(w, map[string]interface{}{"ok": true})

		default:
			http.Error(w, "GET, POST만 받습니다", http.StatusMethodNotAllowed)
		}
	})
}

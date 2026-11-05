package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

// web 폴더(index.html 등)를 exe 안에 같이 넣는다. exe 하나만 옮기면 화면도 따라감
//
//go:embed web
var webFiles embed.FS

// appDir: 설정 파일과 결과 폴더를 둘 기준 폴더.
// 지금 폴더에 config.json이 있으면 지금 폴더(go run으로 개발할 때), 없으면 exe가 있는 폴더
func appDir() string {
	if _, err := os.Stat("config.json"); err == nil {
		if wd, err := os.Getwd(); err == nil {
			return wd
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// runWeb: 이 PC 안에서만 접속할 수 있는 웹 서버를 켜고 브라우저를 연다
func runWeb() {
	dir := appDir()

	// 127.0.0.1:0 = 이 PC에서만 접속 가능 + 비어 있는 포트를 자동으로 고름
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	url := fmt.Sprintf("http://%s/", listener.Addr().String())

	mux := http.NewServeMux()

	// 화면 파일 (web/index.html 등)
	static, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(static)))

	// 상태 확인: 화면이 서버와 연결됐는지, 설정을 어디서 읽는지 보여 줌
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		_, cfgErr := loadConfig(filepath.Join(dir, "config.json"))
		status := map[string]string{
			"appDir":    dir,
			"configErr": "",
		}
		if cfgErr != nil {
			status["configErr"] = cfgErr.Error()
		}
		writeJSON(w, status)
	})

	// 기능 실행(/api/run/○○)과 결과 폴더 열기(/api/open)
	registerJobHandlers(mux, dir)

	// 설정 읽기·저장(/api/config)
	registerConfigHandlers(mux, dir)

	fmt.Println("통합 업무 도구가 실행 중입니다.")
	fmt.Println("브라우저 주소:", url)
	fmt.Println("이 창을 닫으면 프로그램이 종료됩니다.")
	openBrowser(url)

	log.Fatal(http.Serve(listener, mux))
}

// writeJSON: 값을 JSON으로 바꿔서 응답
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// openBrowser: Windows 기본 브라우저로 주소를 연다 (NO_BROWSER 환경 변수가 있으면 열지 않음: 시험용)
func openBrowser(url string) {
	if os.Getenv("NO_BROWSER") != "" {
		return
	}
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
		fmt.Println("브라우저를 열지 못했습니다. 위 주소를 직접 열어 주세요:", err)
	}
}

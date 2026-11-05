package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
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

// 빌드할 때의 config.json을 exe 안에 같이 넣는다. 다른 PC에서 처음 실행하면 이것으로 설정 파일을 만듦
//
//go:embed config.json
var defaultConfig []byte

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

	// 처음 실행이면(설정 파일이 없으면) 기본 설정 파일을 만듦
	created, err := ensureConfig(filepath.Join(dir, "config.json"))
	if err != nil {
		fatal("설정 파일을 만들지 못했습니다 (쓰기 권한이 있는 폴더로 옮겨 주세요)", err)
	}
	if created {
		fmt.Println("처음 실행: 기본 설정 파일을 만들었습니다 →", filepath.Join(dir, "config.json"))
	}

	// 127.0.0.1:0 = 이 PC에서만 접속 가능 + 비어 있는 포트를 자동으로 고름
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal("서버를 켜지 못했습니다", err)
	}
	url := fmt.Sprintf("http://%s/", listener.Addr().String())

	mux := http.NewServeMux()

	// 화면 파일 (web/index.html 등)
	static, err := fs.Sub(webFiles, "web")
	if err != nil {
		fatal("화면 파일을 읽지 못했습니다", err)
	}

	// 어떤 Windows는 레지스트리에 .js가 text/plain으로 잡혀 있어서 모듈 스크립트가 안 뜸 → 직접 지정
	mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
	mime.AddExtensionType(".css", "text/css; charset=utf-8")

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

	fatal("서버가 멈췄습니다", http.Serve(listener, mux))
}

// ensureConfig: 설정 파일이 없으면 exe에 들어 있는 기본 설정으로 만든다. 만들었으면 true
func ensureConfig(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil // 이미 있음
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	return true, os.WriteFile(path, defaultConfig, 0644)
}

// fatal: 오류를 보여 주고 Enter를 누를 때까지 기다렸다가 끝냄
// (exe를 더블클릭해서 띄운 창은 바로 닫히면 오류를 읽을 수 없으므로)
func fatal(msg string, err error) {
	fmt.Println("오류:", msg)
	fmt.Println(err)
	fmt.Println("\nEnter를 누르면 창이 닫힙니다.")
	fmt.Scanln()
	os.Exit(1)
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

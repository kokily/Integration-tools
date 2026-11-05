package web

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 양식 보관 폴더: <작업 폴더>\양식\결산서, <작업 폴더>\양식\소모품
func templateDir(appDir, name string) string {
	return filepath.Join(appDir, "양식", name)
}

// listTemplates: 양식 폴더의 xlsx 파일 이름 목록 (한셀·엑셀이 열어 둘 때 생기는 "~$" 임시 파일은 뺌)
func listTemplates(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.EqualFold(filepath.Ext(name), ".xlsx") || strings.HasPrefix(name, "~$") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// replaceTemplates: 양식 폴더의 기존 xlsx를 지우고 files를 복사해 넣는다
func replaceTemplates(dir string, files []string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, name := range listTemplates(dir) {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	for _, f := range files {
		if err := copyFile(f, filepath.Join(dir, filepath.Base(f))); err != nil {
			return err
		}
	}
	return nil
}

// copyFile: 파일 복사
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// templateInfo: 화면에 보여 줄 보관된 양식 정보
type templateInfo struct {
	Files    []string `json:"files"`
	Modified string   `json:"modified"` // 가장 최근에 바뀐 날짜
}

func readTemplateInfo(dir string) templateInfo {
	info := templateInfo{Files: listTemplates(dir)}
	for _, name := range info.Files {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil {
			if d := st.ModTime().Format("2006-01-02 15:04"); d > info.Modified {
				info.Modified = d
			}
		}
	}
	return info
}

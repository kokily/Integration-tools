package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Save: 지금 설정 파일을 백업 폴더에 복사해 두고, 새 설정을 저장한다
func Save(path, backupDir string, cfg *Config) error {
	if _, err := os.Stat(path); err == nil {
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return err
		}
		backup := filepath.Join(backupDir, "config_"+time.Now().Format("20060102_150405")+".json")
		old, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("백업 실패: %w", err)
		}
		if err := os.WriteFile(backup, old, 0644); err != nil {
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

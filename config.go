package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	WelstoryOrder WelstoryOrderConfig `json:"welstoryOrder"`
}

type WelstoryOrderConfig struct {
	Sites      []string          `json:"sites"`
	Contracts  []string          `json:"contracts"`
	VendorName map[string]string `json:"vendorName"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("설정 파일을 읽을 수 없습니다: %w", err)
	}

	var cfg Config

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("설정 파일 형식 오류 (%s): %w", path, err)
	}

	return &cfg, nil
}

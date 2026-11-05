package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	WelstoryOrder WelstoryOrderConfig `json:"welstoryOrder"`
	Settlement    SettlementConfig    `json:"settlement"`
}

type WelstoryOrderConfig struct {
	Sites      []string          `json:"sites"`
	Contracts  []string          `json:"contracts"`
	VendorName map[string]string `json:"vendorName"`
}

// SettlementConfig: 결산서 설정
type SettlementConfig struct {
	EmployeeSite string            `json:"employeeSite"` // 직원 파일로 나눌 현장명 글자
	RemoveSuffix string            `json:"removeSuffix"` // 현장명에서 떼어 낼 글자
	PlaceMap     map[string]string `json:"placeMap"`     // 현장명 → 납품장소
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

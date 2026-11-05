package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	WelstoryOrder WelstoryOrderConfig `json:"welstoryOrder"`
	Settlement    SettlementConfig    `json:"settlement"`
	Expendables   ExpendablesConfig   `json:"expendables"`
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

// ExpendablesConfig: 소모품 결산 설정
type ExpendablesConfig struct {
	Rates      map[string]float64  `json:"rates"`      // 연도 2자리 → 낙찰률 (예: "26": 0.68375)
	Warehouses map[string][]string `json:"warehouses"` // 결과 시트 이름 → 원본 창고 이름들
	Exclude    []string            `json:"exclude"`    // 결산에서 뺄 원본 창고
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

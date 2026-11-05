package excel

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/xuri/excelize/v2"
)

// 한셀 등이 "x:" 접두어를 붙여 저장한 XML을 일반 형식으로 바꿀 때 쓰는 치환표
var xPrefixReplacer = strings.NewReplacer(
	"<x:", "<",
	"</x:", "</",
	`xmlns:x="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`,
	`xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`,
)

// 한셀 전용 확장(mc:Choice)과 일반 형식(mc:Fallback)을 함께 저장한 묶음 → 일반 형식만 남김
var alternateContentRe = regexp.MustCompile(`(?s)<mc:AlternateContent[^>]*>.*?<mc:Fallback>(.*?)</mc:Fallback>\s*</mc:AlternateContent>`)

// normalizeXML: excelize가 읽을 수 있는 일반 형식으로 정리
func normalizeXML(s string) string {
	s = alternateContentRe.ReplaceAllString(s, "$1")
	// "x:" 접두어 태그가 있을 때만 바꿈 (excelize가 저장한 파일은 이미 일반 형식이라 건드리면 안 됨)
	if strings.Contains(s, "<x:") {
		s = xPrefixReplacer.Replace(s)
	}
	return s
}

// OpenNormalized: xlsx를 열기 전에 xl/ 안 XML을 일반 형식으로 정리한 뒤 excelize로 연다.
// (한셀로 저장한 파일을 그대로 열면 서식이 빠짐)
func OpenNormalized(path string) (*excelize.File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, zf := range zr.File {
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(zf.Name, "xl/") && strings.HasSuffix(zf.Name, ".xml") {
			data = []byte(normalizeXML(string(data)))
		}
		w, err := zw.Create(zf.Name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return excelize.OpenReader(&buf)
}

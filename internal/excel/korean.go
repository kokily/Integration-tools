package excel

import "unicode"

// charRank: 정렬할 때 글자 종류 순위 (기호·공백 < 숫자 < 한글 < 영문). 기존 앱(Windows 한국어 정렬)과 같은 순서
func charRank(r rune) int {
	switch {
	case unicode.IsDigit(r):
		return 1
	case r >= 0xAC00 && r <= 0xD7A3, r >= 0x3131 && r <= 0x318E: // 한글 음절, 자모
		return 2
	case r < 128 && unicode.IsLetter(r): // 영문
		return 3
	}
	return 0
}

// LessKorean: 품명 a가 b보다 앞이면 true
func LessKorean(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if ka, kb := charRank(ra[i]), charRank(rb[i]); ka != kb {
			return ka < kb
		}
		if la, lb := unicode.ToLower(ra[i]), unicode.ToLower(rb[i]); la != lb {
			return la < lb
		}
	}
	return len(ra) < len(rb)
}

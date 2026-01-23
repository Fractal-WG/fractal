package util

func StrPtr(s string) *string { return &s }

func PtrToStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

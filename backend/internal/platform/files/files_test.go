package files

import (
	"errors"
	"testing"
)

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"report.pdf":           "report.pdf",
		"../../etc/passwd":     "passwd",
		`C:\Users\x\notes.md`:  "notes.md",
		"bad\"name\x00.txt":    "badname.txt",
		"":                     "file",
		"  spaced name.docx  ": "spaced name.docx",
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidate(t *testing.T) {
	pdf := []byte("%PDF-1.7\n...")
	zip := []byte("PK\x03\x04rest")
	png := []byte("\x89PNG\r\n\x1a\nrest")
	text := []byte("hello world")
	exe := []byte("MZ\x90\x00binary")

	ok := []struct {
		name string
		head []byte
	}{
		{"a.pdf", pdf}, {"a.PDF", pdf}, {"a.docx", zip}, {"a.zip", zip}, {"a.png", png}, {"a.md", text}, {"a.txt", text},
	}
	for _, c := range ok {
		if _, _, err := Validate(c.name, c.head); err != nil {
			t.Errorf("Validate(%s) = %v", c.name, err)
		}
	}
	bad := []struct {
		name string
		head []byte
	}{
		{"a.exe", exe}, {"a.pdf", exe}, {"a.png", pdf}, {"a.txt", exe}, {"noext", pdf}, {"a.html", []byte("<html>")},
	}
	for _, c := range bad {
		if _, _, err := Validate(c.name, c.head); !errors.Is(err, ErrUnsupportedType) {
			t.Errorf("Validate(%s) should be rejected, got %v", c.name, err)
		}
	}
}

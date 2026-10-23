package domain

import (
	"errors"
	"strings"
	"testing"
)

const validPassword = "correct horse battery"

func TestNormalizeLogin(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		code string
	}{
		{"lowercased", "FlameX", "flamex", ""},
		{"outer spaces removed", "  FLAMEX  ", "flamex", ""},
		{"min length", "abc", "abc", ""},
		{"max length", "a" + strings.Repeat("b", 31), "a" + strings.Repeat("b", 31), ""},
		{"digits and underscore", "user_42", "user_42", ""},
		{"empty", "", "", CodeRequired},
		{"only spaces", "   ", "", CodeRequired},
		{"too short", "ab", "", CodeInvalidLength},
		{"too long", "a" + strings.Repeat("b", 32), "", CodeInvalidLength},
		{"short after trim", " ab ", "", CodeInvalidLength},
		{"first digit", "1abc", "", CodeInvalidCharacters},
		{"first underscore", "_abc", "", CodeInvalidCharacters},
		{"cyrillic", "логин", "", CodeInvalidCharacters},
		{"dot", "fla.mex", "", CodeInvalidCharacters},
		{"hyphen", "fla-mex", "", CodeInvalidCharacters},
		{"inner space", "fla mex", "", CodeInvalidCharacters},
		{"line break", "flamex\n", "", CodeInvalidCharacters},
		{"tab is not trimmed", "\tflamex", "", CodeInvalidCharacters},
		{"non-breaking space is not trimmed", "\u00a0flamex", "", CodeInvalidCharacters},
		{"tab only", "\t", "", CodeInvalidLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeLogin(tt.raw)
			if tt.code == "" {
				if err != nil || got != tt.want {
					t.Fatalf("NormalizeLogin(%q) = %q, %+v; want %q", tt.raw, got, err, tt.want)
				}
				return
			}
			if err == nil || err.Code != tt.code || err.Field != FieldLogin || got != "" {
				t.Fatalf("NormalizeLogin(%q) = %q, %+v; want code %s", tt.raw, got, err, tt.code)
			}
		})
	}
}

func TestNormalizePassword(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		code string
	}{
		{"min length", strings.Repeat("a", 15), strings.Repeat("a", 15), ""},
		{"max length", strings.Repeat("a", 128), strings.Repeat("a", 128), ""},
		{"cyrillic, digits, punctuation, symbols", "Пароль-123, €+ок!", "Пароль-123, €+ок!", ""},
		{"outer spaces kept", " " + validPassword + " ", " " + validPassword + " ", ""},
		{"case kept", "CORRECT horse BATTERY", "CORRECT horse BATTERY", ""},
		{"empty", "", "", CodeRequired},
		{"too short", strings.Repeat("a", 14), "", CodeInvalidLength},
		{"too long is not truncated", strings.Repeat("a", 129), "", CodeInvalidLength},
		{"multibyte counted in code points", strings.Repeat("ж", 14), "", CodeInvalidLength},
		{"tab", validPassword + "\t", "", CodeInvalidCharacters},
		{"line break", validPassword + "\n", "", CodeInvalidCharacters},
		{"non-breaking space", validPassword + "\u00a0", "", CodeInvalidCharacters},
		{"zero width space (format)", validPassword + "\u200b", "", CodeInvalidCharacters},
		{"control character", validPassword + "\x00", "", CodeInvalidCharacters},
		{"invalid UTF-8", validPassword + "\xff", "", CodeInvalidCharacters},
		{"only spaces", strings.Repeat(" ", 15), "", CodeWhitespaceOnly},
		{"short only spaces fails length first", "   ", "", CodeInvalidLength},
		{"long tab fails characters before spaces", strings.Repeat(" ", 14) + "\t", "", CodeInvalidCharacters},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePassword(tt.raw)
			if tt.code == "" {
				if err != nil || got != tt.want {
					t.Fatalf("NormalizePassword = %q, %+v; want %q", got, err, tt.want)
				}
				return
			}
			if err == nil || err.Code != tt.code || err.Field != FieldPassword || got != "" {
				t.Fatalf("NormalizePassword = %q, %+v; want code %s", got, err, tt.code)
			}
		})
	}
}

func TestNormalizePasswordNFC(t *testing.T) {
	composed := strings.Repeat("a", 14) + "\u00e9"    // precomposed e with acute: 15 code points
	decomposed := strings.Repeat("a", 14) + "e\u0301" // e + combining acute: 16 before NFC
	gotComposed, err := NormalizePassword(composed)
	if err != nil {
		t.Fatal(err)
	}
	gotDecomposed, err := NormalizePassword(decomposed)
	if err != nil {
		t.Fatal(err)
	}
	if gotComposed != gotDecomposed || gotDecomposed != composed {
		t.Fatalf("canonically equivalent passwords differ after NFC: %q vs %q", gotComposed, gotDecomposed)
	}

	// 128 code points after NFC are accepted even though the input has 129.
	if _, err := NormalizePassword(strings.Repeat("a", 127) + "e\u0301"); err != nil {
		t.Fatalf("length must be counted after NFC: %+v", err)
	}
	// A combining mark is category M and allowed; after a digit NFC keeps it separate.
	if _, err := NormalizePassword(strings.Repeat("1", 14) + "\u0301"); err != nil {
		t.Fatalf("mark must be allowed: %+v", err)
	}
}

func TestValidateCredentials(t *testing.T) {
	login, password, err := ValidateCredentials(" FlameX ", " "+validPassword)
	if err != nil {
		t.Fatal(err)
	}
	if login != "flamex" || password != " "+validPassword {
		t.Fatalf("got %q, %q", login, password)
	}

	_, _, err = ValidateCredentials("", "short pass")
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	want := []FieldError{
		{Field: FieldLogin, Code: CodeRequired, Message: "Укажите логин"},
		{Field: FieldPassword, Code: CodeInvalidLength, Message: "Пароль должен содержать от 15 до 128 символов"},
	}
	if len(validation.Fields) != len(want) {
		t.Fatalf("fields = %+v, want %+v", validation.Fields, want)
	}
	for i := range want {
		if validation.Fields[i] != want[i] {
			t.Fatalf("fields[%d] = %+v, want %+v", i, validation.Fields[i], want[i])
		}
	}

	_, _, err = ValidateCredentials("flamex", "")
	if !errors.As(err, &validation) || len(validation.Fields) != 1 || validation.Fields[0].Field != FieldPassword {
		t.Fatalf("only the password error is expected: %v", err)
	}
}

func TestPasswordIsNotDisclosed(t *testing.T) {
	secret := "secret password\twith tab"
	_, _, err := ValidateCredentials("flamex", secret)
	if err == nil {
		t.Fatal("expected an error")
	}
	var validation *ValidationError
	errors.As(err, &validation)
	for _, f := range validation.Fields {
		if strings.Contains(f.Message, "secret") {
			t.Fatalf("message repeats the password: %q", f.Message)
		}
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error text repeats the password: %q", err.Error())
	}
}

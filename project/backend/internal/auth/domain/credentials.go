package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Request fields validated by FR-27.
const (
	FieldLogin    = "login"
	FieldPassword = "password"
)

// FieldError codes. The field is named by FieldError.Field.
const (
	CodeRequired          = "REQUIRED"
	CodeInvalidLength     = "INVALID_LENGTH"
	CodeInvalidCharacters = "INVALID_CHARACTERS"
	CodeWhitespaceOnly    = "WHITESPACE_ONLY"
)

// Length limits in Unicode code points: login after trimming U+0020,
// password after NFC normalization.
const (
	LoginMinLength    = 3
	LoginMaxLength    = 32
	PasswordMinLength = 15
	PasswordMaxLength = 128
)

// ValidateCredentials applies the FR-27 rules to both fields and returns the
// normalized login and the NFC password. On failure it returns *ValidationError
// with the first error of each invalid field, login before password.
// Account lookups must happen only after both fields pass.
func ValidateCredentials(login, password string) (string, string, error) {
	normalizedLogin, loginErr := NormalizeLogin(login)
	normalizedPassword, passwordErr := NormalizePassword(password)

	var fields []FieldError
	if loginErr != nil {
		fields = append(fields, *loginErr)
	}
	if passwordErr != nil {
		fields = append(fields, *passwordErr)
	}
	if len(fields) > 0 {
		return "", "", &ValidationError{Fields: fields}
	}
	return normalizedLogin, normalizedPassword, nil
}

// NormalizeLogin removes outer U+0020, checks emptiness, length (3–32 code
// points) and characters ([A-Za-z][A-Za-z0-9_]*), then lowercases the login.
// Tabs, line breaks and other whitespace are not trimmed and are rejected.
func NormalizeLogin(raw string) (string, *FieldError) {
	login := strings.Trim(raw, " ")
	if login == "" {
		return "", loginError(CodeRequired, "Укажите логин")
	}
	if n := utf8.RuneCountInString(login); n < LoginMinLength || n > LoginMaxLength {
		return "", loginError(CodeInvalidLength, "Логин должен содержать от 3 до 32 символов")
	}
	for i := 0; i < len(login); i++ {
		c := login[i]
		letter := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
		if i == 0 && !letter || i > 0 && !letter && !(c >= '0' && c <= '9') && c != '_' {
			return "", loginError(CodeInvalidCharacters, "Логин: первая латинская буква, далее латинские буквы, цифры или _")
		}
	}
	return strings.ToLower(login), nil
}

// NormalizePassword returns the NFC form of the password after checking
// emptiness, length (15–128 code points after NFC), characters (Unicode L, M,
// N, P, S and U+0020) and whitespace-only values. The password is never
// trimmed, case-folded or truncated, and is never included in the error.
func NormalizePassword(raw string) (string, *FieldError) {
	if raw == "" {
		return "", passwordError(CodeRequired, "Укажите пароль")
	}
	password := norm.NFC.String(raw)
	if n := utf8.RuneCountInString(password); n < PasswordMinLength || n > PasswordMaxLength {
		return "", passwordError(CodeInvalidLength, "Пароль должен содержать от 15 до 128 символов")
	}
	for i := 0; i < len(password); {
		r, size := utf8.DecodeRuneInString(password[i:])
		// Invalid UTF-8 decodes as RuneError of size 1; it is not a character.
		if r == utf8.RuneError && size == 1 ||
			r != ' ' && !unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S) {
			return "", passwordError(CodeInvalidCharacters, "Пароль содержит недопустимые символы")
		}
		i += size
	}
	if strings.Trim(password, " ") == "" {
		return "", passwordError(CodeWhitespaceOnly, "Пароль не может состоять только из пробелов")
	}
	return password, nil
}

func loginError(code, message string) *FieldError {
	return &FieldError{Field: FieldLogin, Code: code, Message: message}
}

func passwordError(code, message string) *FieldError {
	return &FieldError{Field: FieldPassword, Code: code, Message: message}
}

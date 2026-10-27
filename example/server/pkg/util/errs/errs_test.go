package errs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func writeProblem(t *testing.T, err error) (int, Problem) {
	t.Helper()
	rec := httptest.NewRecorder()
	NewErrorWriter().WriteError(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil), err)
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var p Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	return rec.Code, p
}

func TestWriteError_BuiltinTypes(t *testing.T) {
	tests := []struct {
		errType ErrorType
		status  int
		title   string
	}{
		{ErrorTypeValidation, http.StatusUnprocessableEntity, "Unprocessable Entity"},
		{ErrorTypeBadRequest, http.StatusBadRequest, "Bad Request"},
		{ErrorTypeNotFound, http.StatusNotFound, "Not Found"},
		{ErrorTypeUnauthorized, http.StatusUnauthorized, "Unauthorized"},
		{ErrorTypeForbidden, http.StatusForbidden, "Forbidden"},
		{ErrorTypeDuplicate, http.StatusConflict, "Conflict"},
		{ErrorTypeConcurrency, http.StatusServiceUnavailable, "Service Unavailable"},
		{ErrorTypeInternal, http.StatusInternalServerError, "Internal Server Error"},
		{ErrorType("UNREGISTERED"), http.StatusInternalServerError, "Internal Server Error"},
	}
	for _, tt := range tests {
		t.Run(string(tt.errType), func(t *testing.T) {
			code, p := writeProblem(t, fmt.Errorf("wrapped: %w", New(tt.errType, "msg")))
			if code != tt.status || p.Status != tt.status || p.Title != tt.title || p.Detail != "msg" || p.Instance != "/api/x" {
				t.Errorf("code = %d, problem = %+v", code, p)
			}
		})
	}
}

func TestWriteError_UnknownErrorIsInternal(t *testing.T) {
	code, p := writeProblem(t, errors.New("boom"))
	if code != http.StatusInternalServerError || p.Title != "Internal Server Error" || p.Detail != "" {
		t.Errorf("code = %d, problem = %+v（内部エラーの詳細を出してはいけない）", code, p)
	}
}

// errorTypeTeapot は使う側と同じ書き方で登録する。パッケージ変数の初期化で Register するので、
// これを使う変数（errTeapot）より必ず先に登録される。
var (
	errorTypeTeapot = Register("TEAPOT", http.StatusTeapot, "I'm a teapot")
	errTeapot       = New(errorTypeTeapot, "short and stout")
)

func TestRegister(t *testing.T) {
	if errTeapot.StatusCode != http.StatusTeapot {
		t.Errorf("StatusCode = %d（パッケージ変数の初期化で作ったエラーに登録が反映されていない）", errTeapot.StatusCode)
	}
	code, p := writeProblem(t, errTeapot)
	if code != http.StatusTeapot || p.Title != "I'm a teapot" {
		t.Errorf("code = %d, problem = %+v", code, p)
	}
}

// stubChecker は、original と同じエラーだけを指定した種類として判定する。
type stubChecker struct {
	original error
	unique   bool
}

func (c stubChecker) IsUniqueConstraintError(err error) bool {
	return c.unique && errors.Is(err, c.original)
}
func (c stubChecker) IsForeignKeyError(error) bool           { return false }
func (c stubChecker) IsNotNullViolationError(error) bool     { return false }
func (c stubChecker) IsCheckViolationError(error) bool       { return false }
func (c stubChecker) IsSerializationFailureError(error) bool { return false }
func (c stubChecker) IsDeadlockDetectedError(error) bool     { return false }
func (c stubChecker) ConstraintName(error) string            { return "" }
func (c stubChecker) ColumnName(error) string                { return "" }

func TestRDBErrorWrapper_KeepsCause(t *testing.T) {
	original := errors.New("pq: connection reset")
	tests := []struct {
		name    string
		checker stubChecker
		errType ErrorType
	}{
		{"unclassified", stubChecker{original: original}, ErrorTypeInternal},
		{"classified", stubChecker{original: original, unique: true}, ErrorTypeDuplicate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			de := NewRDBErrorWrapper(tt.checker).Wrap("user", fmt.Errorf("select: %w", original))
			if de.Type != tt.errType {
				t.Errorf("Type = %s, want %s", de.Type, tt.errType)
			}
			if !errors.Is(de, original) {
				t.Errorf("errors.Is(de, original) = false（元のエラーに辿れない）")
			}
		})
	}
}

func TestRDBErrorWrapper_NotFoundKeepsSentinel(t *testing.T) {
	de := NewRDBErrorWrapper(stubChecker{}).Wrap("user", ErrNotFound)
	if de.Type != ErrorTypeNotFound || !errors.Is(de, ErrNotFound) {
		t.Errorf("Type = %s, errors.Is(de, ErrNotFound) = %v", de.Type, errors.Is(de, ErrNotFound))
	}
}

// captureLog は f の間だけ slog.Default() の出力を取り出す。
func captureLog(t *testing.T, f func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)
	f()
	return buf.String()
}

func TestWriteError_LogsServerErrorsWithCause(t *testing.T) {
	cause := errors.New("pq: connection reset")
	de := NewRDBErrorWrapper(stubChecker{original: cause}).Wrap("user", cause)
	var p Problem
	out := captureLog(t, func() {
		_, p = writeProblem(t, fmt.Errorf("get user: %w", de))
	})
	for _, want := range []string{"level=ERROR", "method=GET", "path=/api/x", "status=500", "get user", "pq: connection reset"} {
		if !strings.Contains(out, want) {
			t.Errorf("log に %q が無い: %s", want, out)
		}
	}
	if strings.Contains(p.Detail, "pq:") {
		t.Errorf("detail = %q（原因をクライアントに出してはいけない）", p.Detail)
	}
}

func TestWriteError_LogsUnknownError(t *testing.T) {
	out := captureLog(t, func() { writeProblem(t, errors.New("boom")) })
	if !strings.Contains(out, "boom") || !strings.Contains(out, "status=500") {
		t.Errorf("log = %q", out)
	}
}

func TestWriteError_DoesNotLogClientErrors(t *testing.T) {
	out := captureLog(t, func() {
		writeProblem(t, NewRDBErrorWrapper(stubChecker{}).Wrap("user", ErrNotFound))
	})
	if out != "" {
		t.Errorf("4xx はログに出さない: %s", out)
	}
}

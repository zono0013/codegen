package errs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

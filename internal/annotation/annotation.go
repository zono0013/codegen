// Package annotation は proto コメント上のアノテーション（@entity / @http など）を解釈する。
// ジェネレータごとに解釈が食い違わないよう、アノテーションの解釈はここに集める。
package annotation

import (
	"fmt"
	"strconv"
	"strings"
)

// HTTP は rpc に付与する `@http METHOD /path`。
type HTTP struct {
	Method string
	Path   string
}

func lines(comment string) []string {
	var out []string
	for _, line := range strings.Split(comment, "\n") {
		out = append(out, strings.TrimSpace(line))
	}
	return out
}

// HasMarker は comment に空白区切りのトークンとして marker（例 "@pk"）が含まれるかを返す。
func HasMarker(comment, marker string) bool {
	for _, line := range lines(comment) {
		for _, tok := range strings.Fields(line) {
			if tok == marker {
				return true
			}
		}
	}
	return false
}

// ParseHTTP は comment から `@http METHOD /path` を取り出す。無い・不正な場合は nil。
func ParseHTTP(comment string) *HTTP {
	for _, line := range lines(comment) {
		if !strings.HasPrefix(line, "@http ") {
			continue
		}
		parts := strings.Fields(strings.TrimPrefix(line, "@http "))
		if len(parts) != 2 {
			continue
		}
		method := strings.ToUpper(parts[0])
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
			return &HTTP{Method: method, Path: parts[1]}
		}
	}
	return nil
}

// PathParams は "/api/orgs/{org_id}/users/{id}" から ["org_id", "id"] を取り出す。
func PathParams(path string) []string {
	var out []string
	for {
		start := strings.Index(path, "{")
		if start == -1 {
			break
		}
		end := strings.Index(path[start:], "}")
		if end == -1 {
			break
		}
		out = append(out, path[start+1:start+end])
		path = path[start+end+1:]
	}
	return out
}

// ParseDecimal は comment から `@decimal(p,s)` を抽出する。値が不正な場合は found=true かつ err!=nil を返す。
func ParseDecimal(comment string) (precision int, scale int, found bool, err error) {
	for _, line := range lines(comment) {
		const prefix = "@decimal("
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(prefix):]
		end := strings.Index(rest, ")")
		if end < 0 {
			return 0, 0, true, fmt.Errorf("@decimal annotation is missing closing %q", ")")
		}
		args := strings.Split(rest[:end], ",")
		if len(args) != 2 {
			return 0, 0, true, fmt.Errorf("@decimal expects exactly 2 arguments (precision, scale), got %d", len(args))
		}
		p, errP := strconv.Atoi(strings.TrimSpace(args[0]))
		if errP != nil {
			return 0, 0, true, fmt.Errorf("@decimal precision %q is not an integer", strings.TrimSpace(args[0]))
		}
		s, errS := strconv.Atoi(strings.TrimSpace(args[1]))
		if errS != nil {
			return 0, 0, true, fmt.Errorf("@decimal scale %q is not an integer", strings.TrimSpace(args[1]))
		}
		if p <= 0 {
			return 0, 0, true, fmt.Errorf("@decimal precision must be positive, got %d", p)
		}
		if s < 0 {
			return 0, 0, true, fmt.Errorf("@decimal scale must be non-negative, got %d", s)
		}
		if s > p {
			return 0, 0, true, fmt.Errorf("@decimal scale (%d) must not exceed precision (%d)", s, p)
		}
		return p, s, true, nil
	}
	return 0, 0, false, nil
}

// Description は comment からアノテーション行（先頭トークンが "@" で始まる行）を除いた説明文を返す。
func Description(comment string) string {
	var out []string
	for _, line := range lines(comment) {
		if strings.HasPrefix(line, "@") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

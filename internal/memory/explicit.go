package memory

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

type explicitPref struct {
	Key      string
	Value    map[string]any
	Priority int
}

// ExplicitLoader 加载 COMPETE.md 显式记忆。
type ExplicitLoader struct {
	path string
}

func NewExplicitLoader(path string) *ExplicitLoader {
	if path == "" {
		path = "COMPETE.md"
	}
	return &ExplicitLoader{path: path}
}

func (l *ExplicitLoader) Load() ([]explicitPref, error) {
	raw, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseExplicitMarkdown(string(raw)), nil
}

func parseExplicitMarkdown(content string) []explicitPref {
	var prefs []explicitPref
	lines := strings.Split(content, "\n")
	keyRe := regexp.MustCompile(`^-\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*:\s*(.+)$`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		m := keyRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		key := strings.TrimSpace(m[1])
		valStr := strings.TrimSpace(m[2])
		val := map[string]any{"text": valStr}
		if strings.HasPrefix(valStr, "[") && strings.HasSuffix(valStr, "]") {
			inner := strings.Trim(valStr, "[]")
			parts := strings.Split(inner, ",")
			arr := make([]string, 0, len(parts))
			for _, p := range parts {
				arr = append(arr, strings.Trim(strings.TrimSpace(p), `"`))
			}
			val = map[string]any{"items": arr}
		} else if n, err := strconv.Atoi(valStr); err == nil {
			val = map[string]any{"value": n}
		}
		prefs = append(prefs, explicitPref{Key: key, Value: val, Priority: 10})
	}
	return prefs
}

// FormatExplicitSection 将显式偏好格式化为 prompt 片段。
func FormatExplicitSection(prefs []Preference) string {
	if len(prefs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 项目显式规则\n")
	for _, p := range prefs {
		b.WriteString("- ")
		b.WriteString(p.Key)
		b.WriteString(": ")
		if t, ok := p.Value["text"].(string); ok {
			b.WriteString(t)
		} else if items, ok := p.Value["items"].([]any); ok {
			for i, it := range items {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(fmtAny(it))
			}
		} else {
			b.WriteString(fmtAny(p.Value))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func fmtAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

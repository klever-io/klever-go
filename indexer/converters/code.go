package converters

import "strings"

func FormatPainlessSource(code string) string {
	formatted := strings.ReplaceAll(code, "\n", "")
	formatted = strings.ReplaceAll(formatted, "\t", "")

	return formatted
}

package collector

import (
	"strconv"
	"strings"
)

func parseRegistryValue(output string) string {
	for _, line := range strings.Split(output, "\n") {
		for _, marker := range []string{"REG_EXPAND_SZ", "REG_MULTI_SZ", "REG_QWORD", "REG_DWORD", "REG_SZ"} {
			if index := strings.Index(line, marker); index >= 0 {
				return strings.TrimSpace(line[index+len(marker):])
			}
		}
	}
	return ""
}

func parseRegistryInteger(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := strconv.ParseUint(value, 0, 64)
	if err != nil {
		return value
	}
	return strconv.FormatUint(parsed, 10)
}

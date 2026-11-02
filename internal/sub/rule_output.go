package sub

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/rules"
)

func clashProxyLines(proxies []map[string]any) []string {
	lines := make([]string, 0, len(proxies)*4)
	for _, proxy := range proxies {
		body, err := marshalClashYAML(proxy)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
			if i == 0 {
				lines = append(lines, "- "+line)
				continue
			}
			lines = append(lines, "  "+line)
		}
	}
	return lines
}

func loadRuleOutput() []rules.OutputRule {
	list, err := rules.LoadOutputRules()
	if err != nil {
		logger.Warning("sub: load routing rules failed:", err)
		return nil
	}
	return list
}

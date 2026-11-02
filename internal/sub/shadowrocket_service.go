package sub

import (
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/rules"
)

// SubShadowrocketService renders the subscription's nodes and rules into a
// complete Shadowrocket `.conf` document.
type SubShadowrocketService struct {
	SubService *SubService
}

func NewSubShadowrocketService(subService *SubService) *SubShadowrocketService {
	return &SubShadowrocketService{SubService: subService}
}

func (s *SubShadowrocketService) GetShadowrocket(subId string, host string, updateURL string) (string, string, error) {
	clash := NewSubClashService(false, "", false, s.SubService)
	proxies, _, header, err := clash.collectProxies(subId, host, false)
	if err != nil {
		return "", "", err
	}
	if len(proxies) == 0 {
		return "", header, nil
	}

	lines := make([]string, 0, len(proxies))
	names := make([]string, 0, len(proxies)+1)
	for _, proxy := range proxies {
		line, name, ok := shadowrocketProxyLine(proxy)
		if !ok {
			continue
		}
		lines = append(lines, line)
		if !isDummyProxy(proxy) || len(proxies) == 1 {
			names = append(names, name)
		}
	}
	if len(lines) == 0 {
		return "", header, nil
	}
	names = append(names, "DIRECT")

	templateText, _ := s.SubService.settingService.GetSubShadowrocketTemplate()
	return rules.RenderShadowrocketConfig(templateText, lines, names, loadRuleOutput(), updateURL), header, nil
}

func srStr(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []string:
		return strings.Join(v, ",")
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	default:
		return ""
	}
}

func srInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func srBool(value any) bool {
	b, _ := value.(bool)
	return b
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// shadowrocketProxyLine converts one Clash proxy map into a Shadowrocket
// `name = type, server, port, ...` line matching rules-manager v3.
func shadowrocketProxyLine(proxy map[string]any) (string, string, bool) {
	name := strings.ReplaceAll(srStr(proxy["name"]), ",", " ")
	server := srStr(proxy["server"])
	port := srInt(proxy["port"])
	kind := srStr(proxy["type"])
	if name == "" || server == "" || port <= 0 || kind == "" {
		return "", "", false
	}

	parts := []string{kind, server, strconv.Itoa(port)}
	add := func(key, value string) {
		if value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	peer := firstNonEmpty(srStr(proxy["servername"]), srStr(proxy["sni"]))

	switch kind {
	case "vless":
		uuid := srStr(proxy["uuid"])
		if uuid == "" {
			return "", "", false
		}
		add("password", uuid)
		if srBool(proxy["tls"]) {
			add("tls", "true")
		}
		add("peer", peer)
		add("flow", srStr(proxy["flow"]))
		add("fp", srStr(proxy["client-fingerprint"]))
		if reality, ok := proxy["reality-opts"].(map[string]any); ok {
			add("pbk", srStr(reality["public-key"]))
			add("sid", srStr(reality["short-id"]))
			add("spx", firstNonEmpty(srStr(proxy["spx"]), "/"))
		}
	case "vmess":
		uuid := srStr(proxy["uuid"])
		if uuid == "" {
			return "", "", false
		}
		add("username", uuid)
		add("vmess-aead", "true")
		if srBool(proxy["tls"]) {
			add("tls", "true")
		}
		add("peer", peer)
		add("fp", srStr(proxy["client-fingerprint"]))
	case "trojan":
		password := srStr(proxy["password"])
		if password == "" {
			return "", "", false
		}
		add("password", password)
		add("peer", peer)
	case "ss":
		add("encrypt-method", srStr(proxy["cipher"]))
		add("password", srStr(proxy["password"]))
	case "hysteria2":
		add("auth", srStr(proxy["password"]))
		add("peer", peer)
		add("obfs", srStr(proxy["obfs"]))
		add("obfsParam", srStr(proxy["obfs-password"]))
	case "tuic":
		add("user", srStr(proxy["uuid"]))
		add("password", srStr(proxy["password"]))
		add("peer", peer)
		add("alpn", srStr(proxy["alpn"]))
		add("congestion-controller", srStr(proxy["congestion-controller"]))
		add("udp-relay-mode", srStr(proxy["udp-relay-mode"]))
	case "socks5":
		return name + " = " + strings.Join(parts, ", "), name, true
	default:
		return "", "", false
	}

	if srBool(proxy["skip-cert-verify"]) {
		add("allowInsecure", "1")
	}

	switch srStr(proxy["network"]) {
	case "ws":
		add("obfs", "websocket")
		if opts, ok := proxy["ws-opts"].(map[string]any); ok {
			add("obfs-path", srStr(opts["path"]))
			if headers, ok := opts["headers"].(map[string]any); ok {
				if host := srStr(headers["Host"]); host != "" {
					add("obfs-header", "Host:"+host)
				}
			}
		}
	case "grpc":
		add("obfs", "grpc")
		if opts, ok := proxy["grpc-opts"].(map[string]any); ok {
			add("obfs-host", srStr(opts["grpc-service-name"]))
		}
	case "h2":
		add("obfs", "http")
	}

	if srBool(proxy["udp"]) {
		add("udp", "1")
	}

	return name + " = " + strings.Join(parts, ", "), name, true
}

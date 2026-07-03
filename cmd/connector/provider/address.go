package provider

import "strings"

const schemeHTTPS = "https"

type normalizedAddress struct {
	host   string
	secure bool
}

func normalizeAddress(raw string, useWss bool) normalizedAddress {
	host := strings.TrimSpace(raw)
	secure := useWss

	if i := strings.Index(host, "://"); i >= 0 {
		s := strings.ToLower(host[:i])
		secure = secure || s == schemeHTTPS || s == wss
		host = host[i+3:]
	}

	host = strings.TrimRight(host, "/")

	return normalizedAddress{host: host, secure: secure}
}

func (n normalizedAddress) httpScheme() string {
	if n.secure {
		return schemeHTTPS
	}
	return "http"
}

func (n normalizedAddress) wsScheme() string {
	if n.secure {
		return wss
	}
	return ws
}

func (n normalizedAddress) statusURL() string {
	return n.httpScheme() + "://" + n.host
}

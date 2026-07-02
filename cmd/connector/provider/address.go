package provider

import "strings"

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

type normalizedAddress struct {
	host   string
	secure bool
}

func normalizeAddress(raw string, useWss bool) normalizedAddress {
	host := strings.TrimSpace(raw)
	secure := useWss

	lower := strings.ToLower(host)
	switch {
	case strings.HasPrefix(lower, schemeHTTPS+"://"):
		host = host[len(schemeHTTPS+"://"):]
		secure = true
	case strings.HasPrefix(lower, schemeHTTP+"://"):
		host = host[len(schemeHTTP+"://"):]
	case strings.HasPrefix(lower, wss+"://"):
		host = host[len(wss+"://"):]
		secure = true
	case strings.HasPrefix(lower, ws+"://"):
		host = host[len(ws+"://"):]
	}

	host = strings.TrimRight(host, "/")

	return normalizedAddress{host: host, secure: secure}
}

func (n normalizedAddress) httpScheme() string {
	if n.secure {
		return schemeHTTPS
	}
	return schemeHTTP
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

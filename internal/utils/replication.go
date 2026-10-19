package utils

import (
	"net"
	"strconv"
	"strings"
	"unicode"
)

func ValidateHostPort(host, port string) bool {
	return validateHost(host) && validatePort(port)
}

func validatePort(port string) bool {
	if port == "" {
		return false
	}
	n, err := strconv.ParseUint(port, 10, 16)
	return err == nil && n > 0
}

func validateHost(host string) bool {
	if host == "" {
		return false
	}

	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.Trim(host, "[]")
	}

	if ip := net.ParseIP(host); ip != nil {
		return true
	}

	if len(host) > 253 {
		return false
	}

	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return false
	}

	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' {
				return false
			}
		}
	}

	return true
}

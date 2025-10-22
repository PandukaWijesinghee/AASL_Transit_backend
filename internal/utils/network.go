package utils

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetRealIP extracts the real client IP address from the request.
// It handles various proxy scenarios and header configurations.
//
// Priority order:
// 1. X-Real-IP header (most specific, set by reverse proxies like Nginx)
// 2. X-Forwarded-For header (comma-separated list, first IP is the client)
//    - Used by Choreo platform and standard load balancers
// 3. Gin's ClientIP() (fallback for direct connections)
//
// Examples:
//   - Direct connection: returns actual IP
//   - Behind Nginx: reads X-Real-IP
//   - Behind Choreo/WSO2: reads first IP from X-Forwarded-For
//   - Behind load balancer: reads first IP from X-Forwarded-For
//   - Development (localhost): returns 127.0.0.1
//
// KNOWN LIMITATION (Choreo Cloud Platform):
// Choreo managed platform may not forward X-Forwarded-For headers.
// In this case, the function returns Choreo's internal proxy IP (10.100.x.x).
// This is a platform limitation - contact Choreo support to enable IP forwarding.
func GetRealIP(c *gin.Context) string {
	// Try X-Real-IP header first (most specific)
	realIP := c.Request.Header.Get("X-Real-IP")
	if realIP != "" && isValidIP(realIP) && !isPrivateIP(net.ParseIP(realIP)) {
		return strings.TrimSpace(realIP)
	}

	// Try X-Forwarded-For header (comma-separated list)
	// Format: X-Forwarded-For: client, proxy1, proxy2
	// We want the first NON-PRIVATE IP (the real client)
	forwarded := c.Request.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Split by comma and get the first valid public IP
		ips := strings.Split(forwarded, ",")
		for _, ipStr := range ips {
			clientIP := strings.TrimSpace(ipStr)
			if isValidIP(clientIP) {
				ip := net.ParseIP(clientIP)
				// Skip private IPs (10.x, 172.16.x, 192.168.x) and use first public IP
				if !isPrivateIP(ip) && !IsLocalhost(clientIP) {
					return clientIP
				}
			}
		}
		// If all IPs are private, return the first valid one
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if isValidIP(clientIP) {
				return clientIP
			}
		}
	}

	// Fallback to Gin's ClientIP (handles RemoteAddr)
	// NOTE: On Choreo cloud platform, this will return internal proxy IP (10.100.x.x)
	// until Choreo enables X-Forwarded-For header forwarding
	return c.ClientIP()
}

// isValidIP checks if the given string is a valid IP address
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// GetUserAgent extracts the User-Agent header from the request
func GetUserAgent(c *gin.Context) string {
	ua := c.Request.UserAgent()
	if ua == "" {
		return "Unknown"
	}
	return ua
}

// IsLocalhost checks if an IP address is localhost
func IsLocalhost(ip string) bool {
	return ip == "127.0.0.1" || ip == "::1" || ip == "localhost"
}

// IPInfo holds information about an IP address
type IPInfo struct {
	IP          string
	IsLocalhost bool
	IsPrivate   bool
	IsValid     bool
}

// GetIPInfo returns detailed information about an IP address
func GetIPInfo(ipStr string) IPInfo {
	info := IPInfo{
		IP:      ipStr,
		IsValid: isValidIP(ipStr),
	}

	if !info.IsValid {
		return info
	}

	ip := net.ParseIP(ipStr)
	info.IsLocalhost = IsLocalhost(ipStr)
	info.IsPrivate = isPrivateIP(ip)

	return info
}

// isPrivateIP checks if an IP is in a private range
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check for private IPv4 ranges
	privateRanges := []string{
		"10.0.0.0/8",     // Class A private
		"172.16.0.0/12",  // Class B private
		"192.168.0.0/16", // Class C private
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

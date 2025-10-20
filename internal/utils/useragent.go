package utils

import (
	"strings"

	ua "github.com/mssola/user_agent"
)

// DeviceInfo holds parsed information from a User-Agent string
type DeviceInfo struct {
	DeviceType string `json:"device_type"` // mobile, tablet, desktop
	OS         string `json:"os"`          // Android 12, iOS 15, Windows 10, etc.
	Browser    string `json:"browser"`     // Chrome, Safari, Firefox, etc.
	BrowserVer string `json:"browser_ver"` // Browser version
	IsBot      bool   `json:"is_bot"`      // Whether it's a bot/crawler
	Platform   string `json:"platform"`    // android, ios, windows, mac, linux
	Raw        string `json:"raw"`         // Original user agent string
}

// ParseUserAgent parses a User-Agent string and extracts device information
func ParseUserAgent(userAgent string) DeviceInfo {
	if userAgent == "" || userAgent == "Unknown" {
		return DeviceInfo{
			DeviceType: "unknown",
			OS:         "Unknown",
			Browser:    "Unknown",
			IsBot:      false,
			Platform:   "unknown",
			Raw:        userAgent,
		}
	}

	// Parse using user_agent library
	parser := ua.New(userAgent)

	deviceInfo := DeviceInfo{
		Raw:    userAgent,
		IsBot:  parser.Bot(),
		OS:     getOS(parser),
		Browser: getBrowser(parser),
		BrowserVer: getBrowserVersion(parser),
		Platform: getPlatform(parser),
	}

	// Determine device type
	deviceInfo.DeviceType = getDeviceType(parser)

	return deviceInfo
}

// getDeviceType determines if the device is mobile, tablet, or desktop
func getDeviceType(parser *ua.UserAgent) string {
	if parser.Mobile() {
		// Check if it's a tablet
		userAgentStr := parser.UA()
		if isTablet(userAgentStr) {
			return "tablet"
		}
		return "mobile"
	}
	return "desktop"
}

// isTablet checks if the user agent indicates a tablet device
func isTablet(userAgent string) bool {
	userAgentLower := strings.ToLower(userAgent)

	tabletIndicators := []string{
		"ipad",
		"tablet",
		"kindle",
		"playbook",
		"nexus 7",
		"nexus 9",
		"nexus 10",
		"xoom",
		"sm-t", // Samsung tablets
		"tab",
	}

	for _, indicator := range tabletIndicators {
		if strings.Contains(userAgentLower, indicator) {
			return true
		}
	}

	return false
}

// getOS extracts operating system name and version
func getOS(parser *ua.UserAgent) string {
	osInfo := parser.OSInfo()
	os := osInfo.Name
	version := osInfo.Version

	if os == "" {
		return "Unknown"
	}

	if version != "" {
		return os + " " + version
	}

	return os
}

// getBrowser extracts browser name
func getBrowser(parser *ua.UserAgent) string {
	name, _ := parser.Browser()
	if name == "" {
		return "Unknown"
	}
	return name
}

// getBrowserVersion extracts browser version
func getBrowserVersion(parser *ua.UserAgent) string {
	_, version := parser.Browser()
	return version
}

// getPlatform determines the platform (android, ios, windows, etc.)
func getPlatform(parser *ua.UserAgent) string {
	osInfo := parser.OSInfo()
	osName := strings.ToLower(osInfo.Name)

	platformMap := map[string]string{
		"android":     "android",
		"ios":         "ios",
		"iphone os":   "ios",
		"windows":     "windows",
		"mac os x":    "mac",
		"macos":       "mac",
		"linux":       "linux",
		"ubuntu":      "linux",
		"chrome os":   "chromeos",
	}

	for key, platform := range platformMap {
		if strings.Contains(osName, key) {
			return platform
		}
	}

	return "unknown"
}

// IsMobileDevice checks if the user agent represents a mobile device
func IsMobileDevice(userAgent string) bool {
	parser := ua.New(userAgent)
	return parser.Mobile()
}

// IsBot checks if the user agent represents a bot/crawler
func IsBot(userAgent string) bool {
	parser := ua.New(userAgent)
	return parser.Bot()
}

// GetDeviceTypeSimple returns a simple device type without full parsing
func GetDeviceTypeSimple(userAgent string) string {
	parser := ua.New(userAgent)
	if parser.Mobile() {
		return "mobile"
	}
	return "desktop"
}

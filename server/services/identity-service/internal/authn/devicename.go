package authn

import "strings"

func parseDeviceName(ua string) string {
	os := parseOS(ua)
	browser := parseBrowser(ua)
	if browser == "" && os == "" {
		return "Unknown device"
	}
	if browser == "" {
		return os
	}
	if os == "" {
		return browser
	}
	return browser + " on " + os
}

func parseOS(ua string) string {
	switch {
	case strings.Contains(ua, "iPhone"):
		return "iPhone"
	case strings.Contains(ua, "iPad"):
		return "iPad"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "Macintosh"), strings.Contains(ua, "Mac OS X"):
		return "macOS"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	default:
		return ""
	}
}

func parseBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "Edg/"):
		return "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		return "Opera"
	case strings.Contains(ua, "Chrome"):
		return "Chrome"
	case strings.Contains(ua, "Firefox"):
		return "Firefox"
	case strings.Contains(ua, "Safari") && !strings.Contains(ua, "Chrome"):
		return "Safari"
	default:
		return ""
	}
}

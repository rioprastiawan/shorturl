package analytics

import "strings"

// Unknown is what every parser returns instead of guessing. A wrong device
// label is worse than an honest gap: it silently skews the breakdown, while
// "unknown" shows up in the chart as exactly what it is.
const Unknown = "unknown"

// Device classes stored in click_events.device (VARCHAR(16)).
const (
	DeviceDesktop = "desktop"
	DeviceMobile  = "mobile"
	DeviceTablet  = "tablet"
	DeviceBot     = "bot"
)

// botMarkers identify non-human traffic. Counting a crawler as a desktop visit
// is the single biggest source of inflated click numbers, so this runs before
// anything else.
var botMarkers = []string{
	"bot", "crawler", "spider", "slurp", "curl", "wget",
	"headless", "python-requests", "go-http-client",
}

// ParseUserAgent classifies a User-Agent header. It is intentionally
// dependency-free and covers only the browsers and platforms that make up
// virtually all real traffic; anything else is reported as unknown.
//
// This runs in the worker, never on the redirect path.
func ParseUserAgent(ua string) (device, os, browser string) {
	lower := strings.ToLower(strings.TrimSpace(ua))
	if lower == "" {
		return Unknown, Unknown, Unknown
	}
	return parseDevice(lower), parseOS(lower), parseBrowser(lower)
}

func parseDevice(ua string) string {
	if containsAny(ua, botMarkers...) {
		return DeviceBot
	}

	switch {
	// iPad first: its User-Agent also contains "Mobile/<build>", so the
	// generic mobile check below would misclassify every iPad as a phone.
	case strings.Contains(ua, "ipad"):
		return DeviceTablet
	case containsAny(ua, "tablet", "kindle", "playbook", "nexus 7", "nexus 10"):
		return DeviceTablet
	// Android phones carry "Mobile"; Android tablets deliberately omit it.
	case strings.Contains(ua, "android"):
		if strings.Contains(ua, "mobile") {
			return DeviceMobile
		}
		return DeviceTablet
	case containsAny(ua, "iphone", "ipod"):
		return DeviceMobile
	case containsAny(ua, "mobile", "windows phone", "iemobile", "opera mini"):
		return DeviceMobile
	case containsAny(ua, "windows", "macintosh", "mac os x", "cros", "x11", "linux"):
		return DeviceDesktop
	}
	return Unknown
}

func parseOS(ua string) string {
	switch {
	// iOS before macOS: an iPad or iPhone UA says "like Mac OS X".
	case containsAny(ua, "iphone", "ipad", "ipod"):
		return "iOS"
	// Android before Linux: every Android UA also says "Linux".
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "windows"):
		return "Windows"
	// ChromeOS before Linux: its UA is "X11; CrOS ...".
	case strings.Contains(ua, "cros"):
		return "ChromeOS"
	case containsAny(ua, "macintosh", "mac os x"):
		return "macOS"
	case containsAny(ua, "linux", "x11"):
		return "Linux"
	}
	return Unknown
}

// parseBrowser walks the derivatives before their base engine. Edge, Opera and
// Samsung Internet all ship Chrome's token, and Chrome itself ships Safari's,
// so a naive ordering reports the entire market as Safari.
func parseBrowser(ua string) string {
	switch {
	case containsAny(ua, "edg/", "edge/", "edga/", "edgios/"):
		return "Edge"
	case containsAny(ua, "opr/", "opera", "opios/"):
		return "Opera"
	case strings.Contains(ua, "samsungbrowser"):
		return "Samsung Internet"
	// fxios is Firefox on iOS, which otherwise looks like mobile Safari.
	case containsAny(ua, "firefox", "fxios"):
		return "Firefox"
	// crios is Chrome on iOS.
	case containsAny(ua, "chrome", "crios", "chromium"):
		return "Chrome"
	case strings.Contains(ua, "safari"):
		return "Safari"
	}
	return Unknown
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

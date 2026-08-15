package analytics

import "testing"

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		name    string
		ua      string
		device  string
		os      string
		browser string
	}{
		// Ordering trap: Edge ships Chrome's token, and Chrome ships Safari's.
		{
			name:    "edge on windows is not chrome",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.2210.91",
			device:  DeviceDesktop,
			os:      "Windows",
			browser: "Edge",
		},
		{
			name:    "chrome on windows is not safari",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			device:  DeviceDesktop,
			os:      "Windows",
			browser: "Chrome",
		},
		{
			name:    "opera is not chrome",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 OPR/105.0.0.0",
			device:  DeviceDesktop,
			os:      "Windows",
			browser: "Opera",
		},
		{
			name:    "samsung internet is not chrome",
			ua:      "Mozilla/5.0 (Linux; Android 13; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/23.0 Chrome/115.0.0.0 Mobile Safari/537.36",
			device:  DeviceMobile,
			os:      "Android",
			browser: "Samsung Internet",
		},
		{
			name:    "real safari on macos",
			ua:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
			device:  DeviceDesktop,
			os:      "macOS",
			browser: "Safari",
		},
		// Ordering trap: an iPad's UA contains both "Mobile/" and "Mac OS X".
		{
			name:    "ipad is a tablet running ios",
			ua:      "Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
			device:  DeviceTablet,
			os:      "iOS",
			browser: "Safari",
		},
		{
			name:    "iphone is mobile running ios",
			ua:      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
			device:  DeviceMobile,
			os:      "iOS",
			browser: "Safari",
		},
		{
			name:    "chrome on ios reports as chrome",
			ua:      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1",
			device:  DeviceMobile,
			os:      "iOS",
			browser: "Chrome",
		},
		{
			name:    "firefox on ios reports as firefox",
			ua:      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/121.0 Mobile/15E148 Safari/605.1.15",
			device:  DeviceMobile,
			os:      "iOS",
			browser: "Firefox",
		},
		// Ordering trap: every Android UA also claims Linux.
		{
			name:    "android phone is not linux",
			ua:      "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			device:  DeviceMobile,
			os:      "Android",
			browser: "Chrome",
		},
		{
			name:    "android without the mobile token is a tablet",
			ua:      "Mozilla/5.0 (Linux; Android 13; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			device:  DeviceTablet,
			os:      "Android",
			browser: "Chrome",
		},
		// Ordering trap: ChromeOS lives behind an X11 token, like Linux.
		{
			name:    "chromeos is not linux",
			ua:      "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			device:  DeviceDesktop,
			os:      "ChromeOS",
			browser: "Chrome",
		},
		{
			name:    "firefox on linux",
			ua:      "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
			device:  DeviceDesktop,
			os:      "Linux",
			browser: "Firefox",
		},
		// Bots are classified before anything else, whatever else they claim.
		{
			name:    "googlebot is a bot even though it claims chrome on android",
			ua:      "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			device:  DeviceBot,
			os:      "Android",
			browser: "Chrome",
		},
		{
			name:    "headless chrome is a bot",
			ua:      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/120.0.0.0 Safari/537.36",
			device:  DeviceBot,
			os:      "Linux",
			browser: "Chrome",
		},
		{
			name:    "curl is a bot with no browser",
			ua:      "curl/8.4.0",
			device:  DeviceBot,
			os:      Unknown,
			browser: Unknown,
		},
		{
			name:    "go http client is a bot",
			ua:      "Go-http-client/2.0",
			device:  DeviceBot,
			os:      Unknown,
			browser: Unknown,
		},
		{
			name:    "python requests is a bot",
			ua:      "python-requests/2.31.0",
			device:  DeviceBot,
			os:      Unknown,
			browser: Unknown,
		},
		{
			name:    "bingbot is a bot",
			ua:      "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
			device:  DeviceBot,
			os:      Unknown,
			browser: Unknown,
		},
		// Nothing recognisable is reported as unknown rather than guessed at.
		{
			name:    "empty user agent",
			ua:      "",
			device:  Unknown,
			os:      Unknown,
			browser: Unknown,
		},
		{
			name:    "whitespace only user agent",
			ua:      "   ",
			device:  Unknown,
			os:      Unknown,
			browser: Unknown,
		},
		{
			name:    "unrecognisable user agent",
			ua:      "SomeInternalTool/1.2",
			device:  Unknown,
			os:      Unknown,
			browser: Unknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			device, os, browser := ParseUserAgent(tc.ua)
			if device != tc.device {
				t.Errorf("device: got %q want %q", device, tc.device)
			}
			if os != tc.os {
				t.Errorf("os: got %q want %q", os, tc.os)
			}
			if browser != tc.browser {
				t.Errorf("browser: got %q want %q", browser, tc.browser)
			}
		})
	}
}

func TestParseUserAgentDeviceValuesFitTheColumn(t *testing.T) {
	for _, device := range []string{DeviceDesktop, DeviceMobile, DeviceTablet, DeviceBot, Unknown} {
		if len(device) > maxDevice {
			t.Errorf("%q is %d bytes, click_events.device holds %d", device, len(device), maxDevice)
		}
	}
}

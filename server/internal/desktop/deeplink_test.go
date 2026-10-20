package desktop

import "testing"

func TestDeepLinksBecomeRoutes(t *testing.T) {
	for link, want := range map[string]string{
		"wapp-debugger://open?project=3&request=42&message=1001": "#/projects/3/requests/42/messages/1001",
		"wapp-debugger://open?request=42":                        "#/requests/42",
		"wapp-debugger://open?server=2":                          "#/servers/2",
		"wapp-debugger://open?project=3&server=2":                "#/projects/3",
		"wapp-debugger://open":                                   "#/",
		"WAPP-DEBUGGER://open/?project=3":                        "#/projects/3",
		"wapp-debugger://open?message=1001":                      "#/",
	} {
		if got, ok := ParseLink(link); !ok || got != want {
			t.Errorf("ParseLink(%q) = %q, %v; want %q", link, got, ok, want)
		}
	}
	for _, link := range []string{
		"https://example.test/open?project=3",
		"wapp-debugger://delete?project=3",
		"wapp-debugger://open?project=-1",
		"wapp-debugger://open?project=x",
		"wapp-debugger://open?request=1e9999",
		"not a url %",
	} {
		if got, ok := ParseLink(link); ok {
			t.Errorf("ParseLink(%q) = %q, accepted", link, got)
		}
	}
}

func TestFindsTheLinkAmongArguments(t *testing.T) {
	if route, ok := LinkInArgs([]string{"/usr/bin/wapp-debugger-desktop", "--db", "x.db", "wapp-debugger://open?request=7"}); !ok || route != "#/requests/7" {
		t.Fatalf("route %q %v", route, ok)
	}
	if _, ok := LinkInArgs([]string{"/usr/bin/wapp-debugger-desktop", "--logs-dir", "/tmp/logs"}); ok {
		t.Fatal("a link out of nothing")
	}
}

func TestNotificationsNeedAnAppBundleOnMacOS(t *testing.T) {
	for _, c := range []struct {
		goos, executable string
		want             bool
	}{
		{"darwin", "/Applications/wapp-debugger.app/Contents/MacOS/wapp-debugger-desktop", true},
		{"darwin", "/Users/dev/dist/wapp-debugger-desktop", false},
		{"linux", "/usr/bin/wapp-debugger-desktop", true},
		{"windows", `C:\Program Files\wapp-debugger\wapp-debugger-desktop.exe`, true},
	} {
		if got := NotificationsAvailable(c.goos, c.executable); got != c.want {
			t.Errorf("NotificationsAvailable(%q, %q) = %v", c.goos, c.executable, got)
		}
	}
}

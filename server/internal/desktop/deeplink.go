package desktop

import (
	"net/url"
	"strconv"
	"strings"
)

// LinkScheme is the URL scheme the installers register: wapp-debugger://open?project=…&request=…&message=…
const LinkScheme = "wapp-debugger"

// ParseLink turns a deep link into a route of the UI (the URL hash). Only "open" with numeric ids
// is understood: a link from a browser or a chat must not do anything else.
func ParseLink(raw string) (string, bool) {
	link, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(link.Scheme, LinkScheme) || link.Host != "open" || strings.Trim(link.Path, "/") != "" {
		return "", false
	}
	query := link.Query()
	id := func(name string) (int64, bool, bool) { // value, present, valid
		value := query.Get(name)
		if value == "" {
			return 0, false, true
		}
		n, err := strconv.ParseInt(value, 10, 64)
		return n, true, err == nil && n > 0
	}
	project, hasProject, okProject := id("project")
	server, hasServer, okServer := id("server")
	request, hasRequest, okRequest := id("request")
	message, hasMessage, okMessage := id("message")
	if !okProject || !okServer || !okRequest || !okMessage {
		return "", false
	}
	route := "#"
	switch {
	case hasProject:
		route += "/projects/" + strconv.FormatInt(project, 10)
	case hasServer:
		route += "/servers/" + strconv.FormatInt(server, 10)
	}
	if hasRequest {
		route += "/requests/" + strconv.FormatInt(request, 10)
		if hasMessage {
			route += "/messages/" + strconv.FormatInt(message, 10)
		}
	}
	if route == "#" {
		route = "#/"
	}
	return route, true
}

// LinkInArgs finds a deep link among the command line arguments: on Windows and Linux the system
// starts the app with the link as an argument.
func LinkInArgs(args []string) (string, bool) {
	for _, arg := range args {
		if strings.HasPrefix(strings.ToLower(arg), LinkScheme+":") {
			return ParseLink(arg)
		}
	}
	return "", false
}

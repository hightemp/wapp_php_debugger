// Package pathmap maps file paths from logs to paths on the developer's machine and to IDE links.
package pathmap

import (
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Project is what the mapping needs from a project.
type Project struct {
	Editor    string // vscode, phpstorm, cursor, sublime or none
	RootPath  string // project root on its server
	LocalPath string // local copy for IDE links; empty means the server path is used
}

// relative returns file relative to the project root, or ok=false for an absolute path outside the root.
// v1 logs hold absolute server paths, v2 logs hold paths relative to the root.
func relative(p Project, file string) (string, bool) {
	if path.IsAbs(file) {
		root := strings.TrimSuffix(p.RootPath, "/")
		if root == "" || !strings.HasPrefix(file, root+"/") {
			return "", false
		}
		file = strings.TrimPrefix(file, root+"/")
	}
	// Clean against a virtual root so "../" cannot climb out of the project
	return strings.TrimPrefix(path.Clean("/"+file), "/"), true
}

// DisplayPath is the path to show: relative to the project root when it is inside it.
func DisplayPath(p Project, file string) string {
	if rel, ok := relative(p, file); ok {
		return rel
	}
	return file
}

// LocalFile is the path of file on the developer's machine.
func LocalFile(p Project, file string) string {
	rel, ok := relative(p, file)
	if !ok {
		return file
	}
	base := p.LocalPath
	if base == "" {
		base = p.RootPath
	}
	return path.Join(base, rel)
}

// EditorURL is a link that opens file at line in the project editor; empty when there is nothing to open.
func EditorURL(p Project, file string, line int) string {
	if file == "" || p.Editor == "none" || p.Editor == "" {
		return ""
	}
	target := LocalFile(p, file)
	suffix := ""
	if line > 0 {
		suffix = ":" + strconv.Itoa(line)
	}
	switch p.Editor {
	case "phpstorm":
		query := url.Values{"file": {target}}
		if line > 0 {
			query.Set("line", strconv.Itoa(line))
		}
		return "phpstorm://open?" + query.Encode()
	case "sublime":
		link := "subl://open?url=" + url.QueryEscape("file://"+target)
		if line > 0 {
			link += "&line=" + strconv.Itoa(line)
		}
		return link
	case "cursor":
		return "cursor://file" + escapePath(target) + suffix
	default:
		return "vscode://file" + escapePath(target) + suffix
	}
}

func escapePath(file string) string {
	segments := strings.Split(file, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

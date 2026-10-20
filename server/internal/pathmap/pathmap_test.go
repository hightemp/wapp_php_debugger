package pathmap

import "testing"

func TestEditorURL(t *testing.T) {
	cases := []struct {
		name string
		p    Project
		file string
		line int
		want string
	}{
		{"v2 relative path, local copy", Project{Editor: "vscode", RootPath: "/var/www/shop", LocalPath: "/home/dev/shop"},
			"app/Model/Item.php", 15, "vscode://file/home/dev/shop/app/Model/Item.php:15"},
		{"v1 absolute path under the project root", Project{Editor: "vscode", RootPath: "/var/www/shop", LocalPath: "/home/dev/shop"},
			"/var/www/shop/app/Model/Item.php", 15, "vscode://file/home/dev/shop/app/Model/Item.php:15"},
		{"no local copy: the path on the server", Project{Editor: "vscode", RootPath: "/var/www/shop"},
			"app/Model/Item.php", 15, "vscode://file/var/www/shop/app/Model/Item.php:15"},
		{"absolute path outside the root stays as is", Project{Editor: "vscode", RootPath: "/var/www/shop", LocalPath: "/home/dev/shop"},
			"/usr/share/php/Lib.php", 3, "vscode://file/usr/share/php/Lib.php:3"},
		{"phpstorm", Project{Editor: "phpstorm", RootPath: "/r", LocalPath: "/home/dev/shop"},
			"a b.php", 7, "phpstorm://open?file=%2Fhome%2Fdev%2Fshop%2Fa+b.php&line=7"},
		{"cursor", Project{Editor: "cursor", RootPath: "/r"}, "x.php", 1, "cursor://file/r/x.php:1"},
		{"sublime", Project{Editor: "sublime", RootPath: "/r"}, "x.php", 1, "subl://open?url=file%3A%2F%2F%2Fr%2Fx.php&line=1"},
		{"no line", Project{Editor: "vscode", RootPath: "/r"}, "x.php", 0, "vscode://file/r/x.php"},
		{"spaces are escaped", Project{Editor: "vscode", RootPath: "/r"}, "dir name/x.php", 2, "vscode://file/r/dir%20name/x.php:2"},
		{"editor none", Project{Editor: "none", RootPath: "/r"}, "x.php", 1, ""},
		{"no file", Project{Editor: "vscode", RootPath: "/r"}, "", 1, ""},
		{"traversal does not escape the root", Project{Editor: "vscode", RootPath: "/var/www/shop", LocalPath: "/home/dev/shop"},
			"../../etc/passwd", 1, "vscode://file/home/dev/shop/etc/passwd:1"},
		{"remote project without a local copy: VS Code Remote-SSH", Project{Editor: "vscode", RootPath: "/var/www/shop", RemoteHost: "deploy@stage-web"},
			"app/Item.php", 15, "vscode://vscode-remote/ssh-remote+deploy@stage-web/var/www/shop/app/Item.php:15"},
		{"remote project in cursor", Project{Editor: "cursor", RootPath: "/r", RemoteHost: "stage"},
			"x.php", 0, "cursor://vscode-remote/ssh-remote+stage/r/x.php"},
		{"remote project with a local copy opens the copy", Project{Editor: "vscode", RootPath: "/r", LocalPath: "/home/dev/r", RemoteHost: "stage"},
			"x.php", 1, "vscode://file/home/dev/r/x.php:1"},
		{"remote project, editor without remote files", Project{Editor: "phpstorm", RootPath: "/r", RemoteHost: "stage"},
			"x.php", 1, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EditorURL(c.p, c.file, c.line); got != c.want {
				t.Fatalf("EditorURL = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDisplayPathIsRelativeToTheRoot(t *testing.T) {
	p := Project{RootPath: "/var/www/shop"}
	for file, want := range map[string]string{
		"/var/www/shop/app/Item.php": "app/Item.php",
		"app/Item.php":               "app/Item.php",
		"/usr/share/php/Lib.php":     "/usr/share/php/Lib.php",
		"/var/www/shopping/x.php":    "/var/www/shopping/x.php",
	} {
		if got := DisplayPath(p, file); got != want {
			t.Errorf("DisplayPath(%q) = %q, want %q", file, got, want)
		}
	}
}

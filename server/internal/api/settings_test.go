package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func validSettings() map[string]any {
	return map[string]any{
		"theme": "custom:paper", "accent_color": "#d14d00", "density": "compact", "ui_font_size": 13,
		"mono_font": "'Iosevka Term', monospace", "env_frame": false, "locale": "ru",
		"custom_themes": []map[string]any{{
			"id": "paper", "name": "Paper", "base": "light",
			"tokens": map[string]string{"bg": "#fdf6e3", "level-error": "#b3261e"},
		}},
	}
}

func TestSettingsHaveDefaults(t *testing.T) {
	e := newTestEnv(t)
	response, body := get(t, e.server.URL+"/api/v1/settings")
	want := `{"accent_color":null,"custom_themes":[],"density":"comfortable","env_frame":true,"locale":null,"mono_font":null,"theme":"system","ui_font_size":14}`
	if response.StatusCode != http.StatusOK || strings.TrimSpace(body) != want {
		t.Fatalf("defaults: %d %s", response.StatusCode, body)
	}
}

func TestSettingsAreSavedAndPublished(t *testing.T) {
	e := newTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := e.broker.Subscribe(ctx, 0)

	response, body := send(t, http.MethodPut, e.server.URL+"/api/v1/settings", validSettings())
	if response.StatusCode != http.StatusOK {
		t.Fatalf("put: %d %s", response.StatusCode, body)
	}
	_, saved := get(t, e.server.URL+"/api/v1/settings")
	var got Settings
	if err := json.Unmarshal([]byte(saved), &got); err != nil || got.Theme != "custom:paper" || got.Density != Compact ||
		len(got.CustomThemes) != 1 || got.CustomThemes[0].Tokens["level-error"] != "#b3261e" {
		t.Fatalf("get: %s", saved)
	}

	select {
	case event := <-stream:
		var payload StreamEvent
		if event.Name != "settings_updated" || json.Unmarshal(event.Data, &payload) != nil || payload.Settings == nil ||
			payload.Settings.UiFontSize != 13 {
			t.Fatalf("event %s %s", event.Name, event.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("no settings_updated")
	}
}

func TestSettingsAreValidated(t *testing.T) {
	e := newTestEnv(t)
	cases := map[string]func(map[string]any){
		"unknown theme":        func(s map[string]any) { s["theme"] = "solarized" },
		"missing custom theme": func(s map[string]any) { s["theme"] = "custom:nord" },
		"custom id bare":       func(s map[string]any) { s["theme"] = "paper" },
		"accent":               func(s map[string]any) { s["accent_color"] = "red" },
		"density":              func(s map[string]any) { s["density"] = "huge" },
		"font size":            func(s map[string]any) { s["ui_font_size"] = 30 },
		"font injection":       func(s map[string]any) { s["mono_font"] = "x; } body { display: none" },
		"locale":               func(s map[string]any) { s["locale"] = "de" },
		"token value": func(s map[string]any) {
			s["custom_themes"].([]map[string]any)[0]["tokens"] = map[string]string{"bg": "url(x)"}
		},
		"token name": func(s map[string]any) {
			s["custom_themes"].([]map[string]any)[0]["tokens"] = map[string]string{"--bg": "#000000"}
		},
		"theme base": func(s map[string]any) { s["custom_themes"].([]map[string]any)[0]["base"] = "system" },
		"same id twice": func(s map[string]any) {
			themes := s["custom_themes"].([]map[string]any)
			s["custom_themes"] = append(themes, themes[0])
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			settings := validSettings()
			change(settings)
			response, body := send(t, http.MethodPut, e.server.URL+"/api/v1/settings", settings)
			assertProblem(t, response, body, http.StatusBadRequest)
		})
	}
	if _, body := get(t, e.server.URL+"/api/v1/settings"); !strings.Contains(body, `"theme":"system"`) {
		t.Fatalf("a refused save changes nothing: %s", body)
	}
}

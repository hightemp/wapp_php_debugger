package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// uiSettingsKey is where the UI settings live in the viewer's database.
const uiSettingsKey = "ui"

var (
	hexColor     = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	builtInTheme = regexp.MustCompile(`^(light|dark|system|high-contrast)$`)
	themeID      = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
	tokenName    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,40}$`)
	// A font-family list: names, quotes, commas; nothing that ends the declaration
	fontFamily = regexp.MustCompile(`^[A-Za-z0-9 ,'_-]{1,200}$`)
)

// DefaultSettings follow the system theme and the browser language.
func DefaultSettings() Settings {
	return Settings{Theme: "system", Density: Comfortable, UiFontSize: 14, EnvFrame: true, CustomThemes: []CustomTheme{}}
}

// GetSettings returns the saved UI settings over the defaults.
func (s *server) GetSettings(ctx context.Context, _ GetSettingsRequestObject) (GetSettingsResponseObject, error) {
	settings := DefaultSettings()
	data, found, err := s.deps.Store.Setting(ctx, uiSettingsKey)
	if err != nil {
		return nil, err
	}
	// A broken value falls back to the defaults: the UI must start
	if found && json.Unmarshal(data, &settings) != nil || validateSettings(settings) != nil {
		settings = DefaultSettings()
	}
	return GetSettings200JSONResponse(settings), nil
}

// UpdateSettings saves the UI settings and tells the other windows.
func (s *server) UpdateSettings(ctx context.Context, request UpdateSettingsRequestObject) (UpdateSettingsResponseObject, error) {
	settings := *request.Body
	if settings.CustomThemes == nil {
		settings.CustomThemes = []CustomTheme{}
	}
	if err := validateSettings(settings); err != nil {
		return UpdateSettingsdefaultApplicationProblemPlusJSONResponse{Body: badRequestProblem(err.Error()), StatusCode: http.StatusBadRequest}, nil
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	if err := s.deps.Store.SetSetting(ctx, uiSettingsKey, data); err != nil {
		return nil, err
	}
	s.deps.Broker.Publish(string(SettingsUpdated), StreamEvent{Event: SettingsUpdated, Settings: &settings})
	return UpdateSettings200JSONResponse(settings), nil
}

// validateSettings checks what the schema says: values end up in CSS of every window.
func validateSettings(settings Settings) error {
	ids := map[string]bool{}
	for i, theme := range settings.CustomThemes {
		if err := validateCustomTheme(theme); err != nil {
			return fmt.Errorf("custom_themes[%d]: %w", i, err)
		}
		if ids[theme.Id] {
			return fmt.Errorf("custom_themes[%d]: id %q is taken", i, theme.Id)
		}
		ids[theme.Id] = true
	}
	switch {
	case len(settings.CustomThemes) > 20:
		return fmt.Errorf("custom_themes: at most 20")
	case !builtInTheme.MatchString(settings.Theme) && !customTheme(settings.Theme, ids):
		return fmt.Errorf("theme: %q is neither built in nor custom:<id> of custom_themes", settings.Theme)
	case settings.AccentColor != nil && !hexColor.MatchString(*settings.AccentColor):
		return fmt.Errorf("accent_color: #rrggbb or null")
	case !settings.Density.Valid():
		return fmt.Errorf("density: compact or comfortable")
	case settings.UiFontSize < 12 || settings.UiFontSize > 18:
		return fmt.Errorf("ui_font_size: from 12 to 18")
	case settings.MonoFont != nil && !fontFamily.MatchString(*settings.MonoFont):
		return fmt.Errorf("mono_font: font names, quotes and commas only")
	case settings.Locale != nil && !settings.Locale.Valid():
		return fmt.Errorf("locale: en, ru or null")
	}
	return nil
}

func customTheme(theme string, ids map[string]bool) bool {
	id, found := strings.CutPrefix(theme, "custom:")
	return found && ids[id]
}

func validateCustomTheme(theme CustomTheme) error {
	switch {
	case !themeID.MatchString(theme.Id):
		return fmt.Errorf("id: lowercase letters, digits and dashes")
	case theme.Name == "" || len([]rune(theme.Name)) > 60:
		return fmt.Errorf("name: from 1 to 60 characters")
	case !theme.Base.Valid():
		return fmt.Errorf("base: light, dark or high-contrast")
	case len(theme.Tokens) > 64:
		return fmt.Errorf("tokens: at most 64")
	}
	for name, value := range theme.Tokens {
		if !tokenName.MatchString(name) || !hexColor.MatchString(value) {
			return fmt.Errorf("tokens: %q must be a token name with a #rrggbb color", name)
		}
	}
	return nil
}

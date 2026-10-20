package desktop

import "github.com/hightemp/wapp_php_debugger/server/internal/api"

// Notification texts in the language of the UI (the settings carry it).
type alertTexts struct {
	errors   func(n int) string
	warnings func(n int) string
	more     string
}

func texts(settings api.DesktopSettings) alertTexts {
	if settings.Locale != nil && *settings.Locale == api.DesktopSettingsLocaleRu {
		return alertTexts{
			errors: func(n int) string { return russian(n, "%d ошибка", "%d ошибки", "%d ошибок") },
			warnings: func(n int) string {
				return russian(n, "%d предупреждение", "%d предупреждения", "%d предупреждений")
			},
			more: "Ещё запросов с ошибками: %d",
		}
	}
	return alertTexts{
		errors:   func(n int) string { return english(n, "%d error", "%d errors") },
		warnings: func(n int) string { return english(n, "%d warning", "%d warnings") },
		more:     "%d more requests with errors",
	}
}

func english(n int, one, other string) string {
	if n == 1 {
		return one
	}
	return other
}

// russian picks the plural form: 1 ошибка, 2 ошибки, 5 ошибок, 21 ошибка.
func russian(n int, one, few, many string) string {
	switch {
	case n%10 == 1 && n%100 != 11:
		return one
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
		return few
	default:
		return many
	}
}

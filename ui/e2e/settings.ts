import type { APIRequestContext } from '@playwright/test';

// Settings live on the viewer and every test of a run shares one: each test starts from the defaults
export const defaultSettings = {
  theme: 'system',
  accent_color: null,
  density: 'comfortable',
  ui_font_size: 14,
  mono_font: null,
  env_frame: true,
  locale: 'en',
  custom_themes: [],
};

export async function resetSettings(request: APIRequestContext, change: object = {}) {
  const response = await request.put('/api/v1/settings', {
    data: { ...defaultSettings, ...change },
  });
  if (!response.ok()) {
    throw new Error(`settings: ${response.status()} ${await response.text()}`);
  }
}

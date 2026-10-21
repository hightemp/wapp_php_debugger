import { describe, expectTypeOf, it } from 'vitest';
import type { ThemeBase } from '../themeTokens';
import { streamEventNames } from './client';
import type { CustomTheme, Level, Request, StreamEvent } from './types';

// Type-level checks: a mismatch with spec/openapi.yaml fails npm run check (svelte-check)
describe('API types from spec/openapi.yaml', () => {
  it('keep the base themes in sync with the settings contract', () => {
    expectTypeOf<CustomTheme['base']>().toEqualTypeOf<ThemeBase>();
  });

  it('listen to every event of the stream', () => {
    expectTypeOf<(typeof streamEventNames)[number]>().toEqualTypeOf<StreamEvent['event']>();
  });

  it('expose message levels from the log format', () => {
    expectTypeOf<Level>().toEqualTypeOf<'error' | 'warning' | 'info' | 'message' | 'debug'>();
  });

  it('keep snake_case field names of DTOs', () => {
    expectTypeOf<Request>().toHaveProperty('status_code');
    expectTypeOf<Request>().toHaveProperty('request_key');
  });
});

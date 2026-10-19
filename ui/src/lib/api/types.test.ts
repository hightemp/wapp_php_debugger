import { describe, expectTypeOf, it } from 'vitest';
import type { ThemePreference } from '../theme';
import type { Level, Request, Settings } from './types';

// Type-level checks: a mismatch with spec/openapi.yaml fails npm run check (svelte-check)
describe('API types from spec/openapi.yaml', () => {
  it('keep theme preferences in sync with the settings contract', () => {
    expectTypeOf<Settings['theme']>().toEqualTypeOf<ThemePreference>();
  });

  it('expose message levels from the log format', () => {
    expectTypeOf<Level>().toEqualTypeOf<'error' | 'warning' | 'info' | 'message' | 'debug'>();
  });

  it('keep snake_case field names of DTOs', () => {
    expectTypeOf<Request>().toHaveProperty('status_code');
    expectTypeOf<Request>().toHaveProperty('request_key');
  });
});

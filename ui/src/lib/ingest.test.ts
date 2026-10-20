import { describe, expect, it } from 'vitest';
import { pushUrl } from './ingest';

describe('pushUrl', () => {
  it('names the receiver as the library will reach it', () => {
    expect(pushUrl('0.0.0.0:8821', 'debug.lan')).toBe('http://debug.lan:8821/ingest/v2');
    expect(pushUrl('[::]:8821', 'debug.lan')).toBe('http://debug.lan:8821/ingest/v2');
    expect(pushUrl(':8821', '')).toBe('http://localhost:8821/ingest/v2');
    expect(pushUrl('10.0.0.5:9000', 'localhost')).toBe('http://10.0.0.5:9000/ingest/v2');
    expect(pushUrl('[fd00::5]:9000', 'localhost')).toBe('http://[fd00::5]:9000/ingest/v2');
    expect(pushUrl(null, 'localhost')).toBe('http://VIEWER_HOST:8821/ingest/v2');
  });
});

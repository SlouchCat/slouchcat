import type { Problem } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { ApiError } from './client';
import { retryAfterSeconds } from './errors';

const locked = (extra: Partial<Problem> = {}): Problem => ({
  type: 'about:blank',
  title: 'pin_locked',
  status: 429,
  code: 'pin_locked',
  ...extra,
});

describe('retryAfterSeconds', () => {
  it('prefers the problem member over the header', () => {
    const error = new ApiError(429, locked({ retryAfterSeconds: 30 }), 60_000);
    expect(retryAfterSeconds(error)).toBe(30);
  });

  it('falls back to Retry-After and rounds up', () => {
    expect(retryAfterSeconds(new ApiError(429, locked(), 12_000))).toBe(12);
    expect(retryAfterSeconds(new ApiError(429, locked(), 1_500))).toBe(2);
  });

  it('is null when the server named no pause or the error is not an API error', () => {
    expect(retryAfterSeconds(new ApiError(429, locked()))).toBeNull();
    expect(retryAfterSeconds(new Error('network'))).toBeNull();
  });
});

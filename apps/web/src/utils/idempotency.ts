import { ApiError } from '@mlf-erp/sdk'

const RETAINED_CODES = new Set([
  'IDEMPOTENCY_CONFLICT',
  'IDEMPOTENCY_IN_PROGRESS',
  'IDEMPOTENCY_RESPONSE_UNAVAILABLE'
])

/**
 * Keep the idempotency key when the outcome is ambiguous or the original
 * request may still be running:
 * - network/timeout failures (the request may have reached the server)
 * - 5xx, 401, 403, 408, 429
 * - conflict / in-progress / response-unavailable
 *
 * Deterministic 4xx failures did not create a side effect. Rotate to a new
 * key before a corrected retry; otherwise the server correctly reports a
 * payload conflict.
 */
export function shouldRetainIdempotencyKey(error: unknown): boolean {
  if (!(error instanceof ApiError)) {
    return true
  }
  if (error.status >= 500) {
    return true
  }
  if (
    error.status === 401 ||
    error.status === 403 ||
    error.status === 408 ||
    error.status === 429
  ) {
    return true
  }
  return RETAINED_CODES.has(error.code)
}

/**
 * Generate a new idempotency key. `crypto.randomUUID()` requires a secure
 * context (HTTPS/localhost), so fall back to `getRandomValues()` for plain
 * HTTP private deployments. Keys only need uniqueness, not secrecy.
 */
export function newIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }

  const bytes = new Uint8Array(16)
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(bytes)
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256)
    }
  }

  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80

  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

import { ApiError } from '@mlf-erp/sdk'

/**
 * Keep the idempotency key only when the outcome is ambiguous:
 * - network/timeout failures (the request may have reached the server)
 * - 5xx responses
 * - a request that is still in progress or whose response cannot be replayed
 *
 * Deterministic 4xx failures did not create a side effect. A corrected retry
 * must use a new key, otherwise the server correctly reports a payload conflict.
 */
export function shouldRetainIdempotencyKey(error: unknown): boolean {
  if (!(error instanceof ApiError)) {
    return true
  }
  if (error.status >= 500) {
    return true
  }
  return (
    error.code === 'IDEMPOTENCY_IN_PROGRESS' ||
    error.code === 'IDEMPOTENCY_RESPONSE_UNAVAILABLE'
  )
}

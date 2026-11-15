/** Extracts a user-facing message from a rejected backend call. */
export function errorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message;
  }
  if (typeof err === 'string') {
    return err;
  }
  return String(err);
}

/** True when the backend call was cancelled (Wails CancelError) rather than failed. */
export function isCancellation(err: unknown): boolean {
  return err instanceof Error && err.name === 'CancelError';
}

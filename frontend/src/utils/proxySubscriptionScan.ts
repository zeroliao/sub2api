export function proxySubscriptionScanErrorCount(
  result?: Record<string, unknown>,
): number {
  const errors = result?.errors;
  if (Array.isArray(errors)) return errors.length;
  return typeof errors === "number" && Number.isFinite(errors) && errors >= 0
    ? errors
    : 0;
}

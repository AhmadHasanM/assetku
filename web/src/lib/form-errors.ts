import type { FieldValues, Path, UseFormSetError } from "react-hook-form"
import { ApiError } from "./api"

/**
 * Copies API field errors (fields.<name> = <code>) onto react-hook-form fields.
 * The code is stored as the error type; render it with useFieldErrorMessage.
 * Returns true when at least one known field was set.
 */
export function applyFieldErrors<T extends FieldValues>(
  err: unknown,
  setError: UseFormSetError<T>,
  known: readonly Path<T>[],
): boolean {
  if (!(err instanceof ApiError)) return false
  let applied = false
  for (const [field, code] of Object.entries(err.fields)) {
    if ((known as readonly string[]).includes(field)) {
      setError(field as Path<T>, { type: code })
      applied = true
    }
  }
  return applied
}

export interface FieldError {
  field: string
  message: string
  meta?: Record<string, unknown>
}

// RFC 7807 Problem Details for HTTP APIs (BE: codegen が生成する pkg/util/errs/http.gen.go)
export interface Problem {
  type: string
  title: string
  status: number
  detail?: string
  instance?: string
  errors?: FieldError[]
}

export function isProblem(error: unknown): error is Problem {
  return (
    error !== null &&
    typeof error === 'object' &&
    'title' in error &&
    'status' in error
  )
}

export function getErrorMessage(
  error: unknown,
  defaultMessage = 'エラーが発生しました'
): string {
  if (isProblem(error)) return error.detail ?? error.title
  if (error instanceof Error) return error.message
  if (typeof error === 'string') return error
  return defaultMessage
}

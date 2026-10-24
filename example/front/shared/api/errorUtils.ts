export interface FieldError {
  field: string
  message: string
  meta?: Record<string, unknown>
}

// RFC 9457 Problem Details for HTTP APIs (BE: codegen が生成する pkg/util/errs/http.gen.go)
export interface Problem {
  type: string
  title: string
  status: number
  detail?: string
  instance?: string
  errors?: FieldError[]
}

// apiClient が失敗時に reject するエラー。Problem の項目をそのまま持つので、isProblem でも扱える
export class ApiError extends Error implements Problem {
  readonly type: string
  readonly title: string
  readonly status: number
  readonly detail?: string
  readonly instance?: string
  readonly errors?: FieldError[]

  constructor(problem: Problem, options?: { cause?: unknown }) {
    super(problem.detail ?? problem.title, options)
    this.name = 'ApiError'
    this.type = problem.type
    this.title = problem.title
    this.status = problem.status
    this.detail = problem.detail
    this.instance = problem.instance
    this.errors = problem.errors
  }
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

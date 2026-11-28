import type { TheoryAnswer } from '@chaimir/api-client'

export function isTheoryAnswerEmpty(value: TheoryAnswer | undefined): boolean {
  if (value === undefined) return true
  if (typeof value === 'boolean') return false
  if (Array.isArray(value)) return value.every((entry) => entry.trim() === '')
  return value.trim() === ''
}

export function isAutomaticTheoryQuestion(body: Record<string, unknown>): boolean {
  switch (body.q_type) {
    case 'single_choice':
    case 'multiple_choice':
    case 'true_false':
    case 'fill_blank':
      return true
    default:
      return false
  }
}

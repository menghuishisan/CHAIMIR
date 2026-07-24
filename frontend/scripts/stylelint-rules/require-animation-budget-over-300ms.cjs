/**
 * Stylelint rule that requires an explicit reason for long animations.
 */

function hasBudgetComment(decl) {
  let previous = decl.prev()
  while (previous && previous.type === 'comment') {
    if (/animation-budget\s*:/i.test(previous.text)) return true
    previous = previous.prev()
  }
  return false
}

function hasLongDuration(value) {
  return [...value.matchAll(/(\d+(?:\.\d+)?)(ms|s)\b/gi)].some((match) => {
    const milliseconds = match[2].toLowerCase() === 's' ? Number(match[1]) * 1000 : Number(match[1])
    return milliseconds > 300
  })
}

module.exports = function requireAnimationBudget({ root, result, ruleName, stylelint }) {
  root.walkDecls(/^(?:transition|transition-duration|animation|animation-duration)$/i, (decl) => {
    if (!hasLongDuration(decl.value) || hasBudgetComment(decl)) return
    if (/animation\s*:/i.test(decl.prop) && /\binfinite\b/i.test(decl.value) && /\b(?:spin|loader|loading)\b/i.test(decl.value)) return

    stylelint.utils.report({
      message: 'Animation durations over 300ms require an /* animation-budget: reason */ comment.',
      node: decl,
      result,
      ruleName,
    })
  })
}

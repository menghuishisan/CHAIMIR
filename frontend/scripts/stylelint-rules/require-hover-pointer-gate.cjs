/**
 * Stylelint rule that keeps hover-only states on fine pointers.
 */

module.exports = function requireHoverPointerGate({ root, result, ruleName, stylelint }) {
  root.walkRules((rule) => {
    if (!rule.selector.includes(':hover')) return

    let parent = rule.parent
    let gated = false
    while (parent && parent.type !== 'root') {
      if (parent.type === 'atrule' && parent.name.toLowerCase() === 'media') {
        gated = /\(\s*hover\s*:\s*hover\s*\)/i.test(parent.params)
          && /\(\s*pointer\s*:\s*fine\s*\)/i.test(parent.params)
        if (gated) break
      }
      parent = parent.parent
    }

    if (!gated) {
      stylelint.utils.report({
        message: ':hover selectors must be nested under @media (hover: hover) and (pointer: fine).',
        node: rule,
        result,
        ruleName,
      })
    }
  })
}

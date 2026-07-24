/**
 * Stylelint rule that requires the shared custom-media breakpoint contract.
 */

module.exports = function allowedMediaQueries({ root, result, ruleName, stylelint }) {
  root.walkAtRules('media', (atRule) => {
    const params = atRule.params.trim()
    if (/\b(?:min|max)-width\s*:/i.test(params)) {
      stylelint.utils.report({
        message: 'Viewport breakpoints must use the shared --bp-sm/--bp-md/--bp-lg/--bp-xl/--bp-2xl custom media.',
        node: atRule,
        result,
        ruleName,
      })
    }
  })
}

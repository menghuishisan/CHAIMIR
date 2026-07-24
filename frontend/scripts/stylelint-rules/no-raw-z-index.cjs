/**
 * Stylelint rule that keeps stacking order on the shared z-index scale.
 */

module.exports = function noRawZIndex({ root, result, ruleName, stylelint }) {
  const file = root.source?.input?.file?.replaceAll('\\', '/') ?? ''
  if (file.includes('/tokens/')) return

  root.walkDecls(/^z-index$/i, (decl) => {
    const value = decl.value.trim()
    if (/^var\(--z-[\w-]+\)|^calc\(\s*var\(--z-[\w-]+\)/i.test(value)) return
    if (/^(?:auto|inherit|initial|revert|revert-layer|unset)$/i.test(value)) return

    stylelint.utils.report({
      message: 'z-index must use a --z-* token.',
      node: decl,
      result,
      ruleName,
    })
  })
}

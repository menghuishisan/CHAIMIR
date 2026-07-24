/**
 * ESLint rule that rejects emoji in source strings and JSX text.
 */

const emojiPattern = /[\u{1F1E6}-\u{1F1FF}\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u

module.exports = {
  meta: { type: 'problem', docs: { description: 'Disallow emoji in frontend source.' }, schema: [] },
  create(context) {
    const report = (node) => {
      const value = node.type === 'JSXText' ? node.value : node.value?.raw ?? node.value
      if (typeof value === 'string' && emojiPattern.test(value)) {
        context.report({ node, message: 'Emoji are forbidden in frontend UI, copy, and source.' })
      }
    }
    return {
      Literal: report,
      TemplateElement: report,
      JSXText: report,
    }
  },
}

/**
 * Single Stylelint source of truth for all Chaimir frontend workspaces.
 */

module.exports = {
  plugins: [require('./stylelint-rules/index.cjs')],
  rules: {
    'chaimir/gates': true,
    'at-rule-no-unknown': [true, { ignoreAtRules: ['custom-media'] }],
    'block-no-empty': true,
    'color-no-invalid-hex': true,
    'declaration-block-no-duplicate-properties': [true, { ignore: ['consecutive-duplicates-with-different-values'] }],
    'declaration-block-no-shorthand-property-overrides': true,
    'declaration-property-value-no-unknown': [true, { ignoreProperties: { r: [/.*/] } }],
    'declaration-property-value-allowed-list': { 'letter-spacing': ['0'] },
    'media-feature-name-no-unknown': true,
    'no-descending-specificity': null,
    'property-no-unknown': true,
    'selector-pseudo-class-no-unknown': [true, { ignorePseudoClasses: ['global'] }],
  },
}

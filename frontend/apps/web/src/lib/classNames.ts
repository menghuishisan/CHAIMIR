// 类名工具：合并字符串、数组和对象形式的条件类名。

type ClassDictionary = Record<string, boolean | null | undefined>
type ClassValue = string | number | boolean | undefined | null | ClassValue[] | ClassDictionary

export function cn(...inputs: ClassValue[]): string {
  const classes: string[] = []

  for (const input of inputs) {
    if (!input) continue

    if (typeof input === 'string' || typeof input === 'number') {
      classes.push(String(input))
      continue
    }

    if (Array.isArray(input)) {
      const result = cn(...input)
      if (result) {
        classes.push(result)
      }
      continue
    }

    if (typeof input === 'object') {
      for (const [className, enabled] of Object.entries(input)) {
        if (enabled) {
          classes.push(className)
        }
      }
    }
  }

  return classes.join(' ')
}

// CompositionParameterFields 只渲染目录声明的参数控件,校验和映射统一由M2执行。
import { FormField, Input, Select } from '@chaimir/ui'

interface CompositionParameterFieldsProps {
  idPrefix: string
  schema?: Record<string, unknown> | null
  value?: Record<string, unknown>
  onChange: (value: Record<string, unknown>) => void
}

/** parameterDescription 只读取目录中的用户向说明,不展示内部环境变量映射。 */
function parameterDescription(property: Record<string, unknown>): string | undefined {
  return typeof property.description === 'string' ? property.description : undefined
}

/** CompositionParameterFields 复用基础控件呈现当前目录标量,保留未设置、0与false的区别。 */
export function CompositionParameterFields({ idPrefix, schema, value = {}, onChange }: CompositionParameterFieldsProps) {
  const properties = schema?.properties
  if (typeof properties !== 'object' || properties === null || Array.isArray(properties)) return null
  const required = Array.isArray(schema?.required) ? schema.required : []
  return (
    <div className="flex flex-col gap-3">
      {Object.entries(properties).map(([name, definition], index) => {
        if (typeof definition !== 'object' || definition === null || Array.isArray(definition)) return null
        const property = definition as Record<string, unknown>
        const label = typeof property.title === 'string' && property.title.trim() !== '' ? property.title : `配置项 ${index + 1}`
        const fieldId = `${idPrefix}-parameter-${index}`
        const isRequired = required.includes(name)
        const current = value[name]
        // 空输入撤销该参数,不把明确的0或false当成未设置。
        const setValue = (next: unknown) => {
          const updated = { ...value }
          if (next === undefined) delete updated[name]
          else updated[name] = next
          onChange(updated)
        }
        const choices: unknown[] | undefined = Array.isArray(property.enum)
          ? property.enum
          : property.type === 'boolean' ? [true, false] : undefined
        const selectedIndex = choices?.findIndex((choice) => Object.is(choice, current)) ?? -1
        return (
          <FormField key={name} label={label} htmlFor={fieldId} required={isRequired} helper={parameterDescription(property)}>
            {choices !== undefined ? (
              <Select
                id={fieldId}
                value={selectedIndex >= 0 ? String(selectedIndex) : ''}
                placeholder={isRequired ? '请选择' : '保持未设置'}
                options={[
                  ...(!isRequired ? [{ label: '保持未设置', value: 'unset' }] : []),
                  ...choices.map((choice, choiceIndex) => ({
                    label: typeof choice === 'boolean' ? choice ? '是' : '否' : String(choice),
                    value: String(choiceIndex),
                  })),
                ]}
                onValueChange={(selected) => setValue(selected === 'unset' ? undefined : choices[Number(selected)])}
              />
            ) : (
              <Input
                id={fieldId}
                type={property.type === 'integer' || property.type === 'number' ? 'number' : 'text'}
                value={typeof current === 'string' || typeof current === 'number' ? current : ''}
                required={isRequired}
                min={typeof property.minimum === 'number' ? property.minimum : undefined}
                max={typeof property.maximum === 'number' ? property.maximum : undefined}
                step={property.type === 'integer' ? 1 : 'any'}
                minLength={typeof property.minLength === 'number' ? property.minLength : undefined}
                maxLength={typeof property.maxLength === 'number' ? property.maxLength : undefined}
                pattern={typeof property.pattern === 'string' ? property.pattern : undefined}
                onChange={(event) => {
                  const input = event.currentTarget
                  if (input.value === '') setValue(undefined)
                  else if (input.type === 'number' && Number.isFinite(input.valueAsNumber)) setValue(input.valueAsNumber)
                  else setValue(input.value)
                }}
              />
            )}
          </FormField>
        )
      })}
    </div>
  )
}

// 下载文件名：集中维护浏览器保存文件时展示给用户的名称。

type AccountImportTarget = 'teacher' | 'student'

export const DOWNLOAD_FILENAMES = {
  ACCOUNT_IMPORT_TEMPLATE: {
    teacher: '教师账号导入模板.xlsx',
    student: '学生账号导入模板.xlsx',
  },
  ORG_IMPORT_TEMPLATE: '组织架构导入模板.xlsx',
} as const

export function accountImportTemplateFilename(target: AccountImportTarget): string {
  return DOWNLOAD_FILENAMES.ACCOUNT_IMPORT_TEMPLATE[target]
}

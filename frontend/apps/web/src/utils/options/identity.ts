// identity 定义账号、组织与认证页面使用的选择项。

import { AccountStatus, AuthMode, BaseIdentity, ClassStatus, ImportTarget, SsoMatchField, SsoType, UserRole } from '@chaimir/api-client'
import { accountStatusLabel, authModeLabel, baseIdentityLabel, classStatusLabel, ssoMatchFieldLabel, ssoTypeLabel } from '../labels'
import { option, withAllOption } from './shared'

export const accountRoleFilterOptions = withAllOption('全部角色', [option(UserRole.STUDENT, '学生'), option(UserRole.TEACHER, '教师'), option(UserRole.SCHOOL_ADMIN, '学校管理员')])
export const announcementTargetRoleOptions = [option(UserRole.STUDENT, '学生'), option(UserRole.TEACHER, '教师'), option(UserRole.SCHOOL_ADMIN, '学校管理员')]
export const accountStatusFilterOptions = withAllOption('全部状态', [option(AccountStatus.PENDING, accountStatusLabel(AccountStatus.PENDING)), option(AccountStatus.ACTIVE, accountStatusLabel(AccountStatus.ACTIVE)), option(AccountStatus.DISABLED, accountStatusLabel(AccountStatus.DISABLED)), option(AccountStatus.ARCHIVED, accountStatusLabel(AccountStatus.ARCHIVED))])
export const baseIdentityOptions = [option(BaseIdentity.STUDENT, baseIdentityLabel(BaseIdentity.STUDENT)), option(BaseIdentity.TEACHER, baseIdentityLabel(BaseIdentity.TEACHER))]
export const importTargetOptions = [option(ImportTarget.TEACHER, '教师账号'), option(ImportTarget.STUDENT, '学生账号'), option(ImportTarget.ORG, '组织架构')]
export const accountImportTargetOptions = [option('student', '学生账号'), option('teacher', '教师账号')]
export const tenantApplicationSchoolTypeOptions = [option(1, '本科院校'), option(2, '高职高专'), option(3, '其他教育机构')]
export const authModeOptions = [option(AuthMode.LOCAL, authModeLabel(AuthMode.LOCAL)), option(AuthMode.CAS, authModeLabel(AuthMode.CAS)), option(AuthMode.LDAP, authModeLabel(AuthMode.LDAP))]
export const ssoTypeOptions = [option(SsoType.CAS, ssoTypeLabel(SsoType.CAS)), option(SsoType.LDAP, ssoTypeLabel(SsoType.LDAP))]
export const ssoMatchFieldOptions = [option(SsoMatchField.NO, ssoMatchFieldLabel(SsoMatchField.NO)), option(SsoMatchField.PHONE, ssoMatchFieldLabel(SsoMatchField.PHONE))]
export const classStatusOptions = [option(ClassStatus.ACTIVE, classStatusLabel(ClassStatus.ACTIVE)), option(ClassStatus.ARCHIVED, classStatusLabel(ClassStatus.ARCHIVED))]

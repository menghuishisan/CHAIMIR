// organizationTree.ts 组织树的纯逻辑(教师组织查看 / 校管组织架构共用,设计 §7.4 / §7.5):
// 院系→专业树按搜索词收窄(院系名或代码、专业名、班级名),地址里的选中节点不存在时回到默认,
// 专业下的班级按在读在前、入学年份新的在前排序。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 班级状态取值,与 @chaimir/api-client 的 ClassStatus 一致 */
const CLASS_ACTIVE = 1

/** 树用到的院系字段 */
export interface TreeDepartment {
  id: string
  name: string
  code: string
}

/** 树用到的专业字段 */
export interface TreeMajor {
  id: string
  department_id: string
  name: string
}

/** 树用到的班级字段 */
export interface TreeClass {
  id: string
  major_id: string
  name: string
  enrollment_year: number
  status: number
}

export interface MajorNode<M extends TreeMajor> {
  major: M
  /** 这个专业下的班级总数(含已归档) */
  classCount: number
}

export interface DepartmentNode<D extends TreeDepartment, M extends TreeMajor> {
  department: D
  majors: MajorNode<M>[]
}

/** 当前选中的节点 */
export type OrgSelection =
  | { kind: 'department'; departmentId: string }
  | { kind: 'major'; departmentId: string; majorId: string }

/** matches 不区分大小写的包含判断 */
function matches(text: string, needle: string): boolean {
  return text.toLowerCase().includes(needle)
}

/**
 * buildOrgTree 组装院系→专业树并按搜索词收窄:院系名或代码命中时带上它的全部专业;
 * 否则只留名字命中或下属班级名命中的专业,一个都没有的院系不显示。
 */
export function buildOrgTree<D extends TreeDepartment, M extends TreeMajor, C extends TreeClass>(
  departments: readonly D[],
  majors: readonly M[],
  classes: readonly C[],
  keyword: string,
): DepartmentNode<D, M>[] {
  const needle = keyword.trim().toLowerCase()
  const classCount = new Map<string, number>()
  const classNames = new Map<string, string[]>()
  for (const entity of classes) {
    classCount.set(entity.major_id, (classCount.get(entity.major_id) ?? 0) + 1)
    classNames.set(entity.major_id, [...(classNames.get(entity.major_id) ?? []), entity.name])
  }
  const nodes: DepartmentNode<D, M>[] = []
  for (const department of departments) {
    const own = majors
      .filter((major) => major.department_id === department.id)
      .map((major) => ({ major, classCount: classCount.get(major.id) ?? 0 }))
    if (needle === '' || matches(department.name, needle) || matches(department.code, needle)) {
      nodes.push({ department, majors: own })
      continue
    }
    const kept = own.filter(
      (node) => matches(node.major.name, needle) || (classNames.get(node.major.id) ?? []).some((name) => matches(name, needle)),
    )
    if (kept.length > 0) nodes.push({ department, majors: kept })
  }
  return nodes
}

/**
 * resolveOrgSelection 把地址里的选中节点对到树上:专业优先,其次院系;都不在树上(被删或被搜索收窄)时
 * 选第一个有专业的院系的第一个专业,再不行选第一个院系;树为空时返回 undefined。
 */
export function resolveOrgSelection<D extends TreeDepartment, M extends TreeMajor>(
  tree: readonly DepartmentNode<D, M>[],
  wanted: { dept: string; major: string },
): OrgSelection | undefined {
  if (wanted.major) {
    for (const node of tree) {
      if (node.majors.some((child) => child.major.id === wanted.major)) {
        return { kind: 'major', departmentId: node.department.id, majorId: wanted.major }
      }
    }
  }
  if (wanted.dept && tree.some((node) => node.department.id === wanted.dept)) {
    return { kind: 'department', departmentId: wanted.dept }
  }
  const withMajor = tree.find((node) => node.majors.length > 0)
  if (withMajor) return { kind: 'major', departmentId: withMajor.department.id, majorId: withMajor.majors[0].major.id }
  return tree.length > 0 ? { kind: 'department', departmentId: tree[0].department.id } : undefined
}

/**
 * classesOfMajor 一个专业下的班级:在读在前、入学年份新的在前、再按名称;
 * 搜索词只命中其中一部分班级名时只列命中的(filtered 为 true),否则列全部。
 */
export function classesOfMajor<C extends TreeClass>(
  classes: readonly C[],
  majorId: string,
  keyword: string,
): { list: C[]; filtered: boolean } {
  const needle = keyword.trim().toLowerCase()
  const all = classes
    .filter((entity) => entity.major_id === majorId)
    .sort(
      (a, b) =>
        Number(b.status === CLASS_ACTIVE) - Number(a.status === CLASS_ACTIVE) ||
        b.enrollment_year - a.enrollment_year ||
        a.name.localeCompare(b.name, 'zh-CN'),
    )
  if (needle === '') return { list: all, filtered: false }
  const hit = all.filter((entity) => matches(entity.name, needle))
  return hit.length > 0 && hit.length < all.length ? { list: hit, filtered: true } : { list: all, filtered: false }
}

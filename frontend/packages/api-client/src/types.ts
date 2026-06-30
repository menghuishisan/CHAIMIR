// 类型定义：与后端 DTO 对齐

// ===== 通用类型 =====

export interface PaginatedResponse<T> {
  list: T[]
  total: number
  page: number
  size: number
}

// ===== M1 Identity 模块 =====

export interface LoginPlatformRequest {
  username: string
  password: string
}

export interface LoginPhoneRequest {
  phone: string
  password: string
  tenant_id: number
}

export interface LoginNoRequest {
  tenant_code: string
  no: string
  password: string
}

export interface LoginSMSRequest {
  phone: string
  code: string
  tenant_id: number
}

export interface SendSMSRequest {
  phone: string
  scene: number
  tenant_id: number
}

export interface RefreshRequest {
  refresh_token: string
}

export interface PasswordResetRequest {
  phone: string
  code: string
  new_password: string
  tenant_id: number
}

export interface ActivateRequest {
  activation_code: string
  password: string
}

export interface LoginResponse {
  access_token?: string
  refresh_token?: string
  must_change_pwd?: boolean
  need_select_tenant?: boolean
  tenants?: TenantOption[]
  account?: Account
}

export interface TenantOption {
  tenant_id: string
  name: string
  code: string
}

export interface Account {
  id: string
  tenant_id: string
  name: string
  phone_masked?: string
  no?: string
  base_identity: number
  roles: number[]
  status: number
  title?: string
  created_at?: string
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

export interface ChangePhoneRequest {
  phone: string
  code: string
}

export interface Session {
  id: string
  device_info?: string
  ip?: string
  status: number
  expire_at: string
  created_at: string
}

export interface AuditLog {
  id: string
  tenant_id?: string
  actor_id: string
  actor_role: number
  action: string
  target_type: string
  target_id?: string
  detail?: string
  ip?: string
  trace_id?: string
  created_at: string
}

export interface CreateApplicationRequest {
  school_name: string
  school_type: number
  contact_name: string
  contact_phone: string
  contact_email: string
}

export interface Tenant {
  id: string
  code: string
  name: string
  type: number
  status: number
  deploy_mode: number
  expire_at?: string
  logo_url?: string
  display_name?: string
  auth_mode: number
  enable_activation_code: boolean
}

// ===== M5 Content 模块 =====

export interface ContentItem {
  id: string
  tenant_id: string
  code: string
  version: string
  type: number
  title: string
  category_id?: number
  difficulty: number
  tags: string[]
  knowledge_points: string[]
  author_id: string
  author_type: number
  visibility: number
  status: number
  usage_count: number
  version_hash: string
  created_at: string
  updated_at: string
}

export interface ContentItemSnapshot extends ContentItem {
  body: Record<string, any>
  sensitive_fields?: string[]
}

export interface CreateItemRequest {
  code: string
  version: string
  type: number
  title: string
  category_id: number
  difficulty: number
  tags: string[]
  knowledge_points: string[]
  visibility: number
  body: Record<string, any>
  sensitive_fields: string[]
}

export interface UpdateItemRequest {
  title: string
  category_id: number
  difficulty: number
  tags: string[]
  knowledge_points: string[]
  visibility: number
  body: Record<string, any>
  sensitive_fields: string[]
}

export interface NewVersionRequest {
  source_version: string
  new_version: string
}

export interface CloneItemRequest {
  new_code: string
  new_version: string
}

export interface ContentCategory {
  id: string
  parent_id?: string
  name: string
  sort: number
  created_at: string
  updated_at: string
}

export interface ContentCategoryRequest {
  parent_id: number
  name: string
  sort: number
}

export interface ContentAttachmentUpload {
  object_ref: string
  file_name: string
  size: number
}

export interface ContentAttachmentDownloadGrantRequest {
  resource_id: string
  object_ref: string
}

export interface ContentAttachmentDownloadGrant {
  token: string
  expires_at: string
}

export interface PaperCriteria {
  type?: number
  difficulty?: number[]
  knowledge_points?: string[]
  count?: number
  default_score?: number
}

export interface PaperItemInput {
  code: string
  version: string
  score: number
}

export interface CreatePaperRequest {
  name: string
  gen_mode: number
  gen_criteria: PaperCriteria
  items: PaperItemInput[]
}

export interface Paper {
  id: string
  name: string
  author_id: string
  gen_mode: number
  gen_criteria: PaperCriteria
  created_at: string
  updated_at: string
}

export interface PaperItemFace {
  id: string
  code: string
  version: string
  score: number
  seq: number
  item: ContentItem
  body: Record<string, any>
}

export interface PaperDetail {
  paper: Paper
  items: PaperItemFace[]
}

// ===== M6 Teaching 模块 =====

export interface Course {
  id: string
  tenant_id: string
  teacher_id: string
  name: string
  description: string
  type: number
  difficulty: number
  cover_url?: string
  semester: string
  credits: number
  schedule: Record<string, any>
  start_at: string
  end_at: string
  invite_code?: string
  status: number
  visibility: number
  created_at: string
  updated_at: string
}

export interface CourseRequest {
  name: string
  description: string
  type: number
  difficulty: number
  cover_url?: string
  semester: string
  credits: number
  schedule: Record<string, any>
  start_at: string
  end_at: string
}

export interface Chapter {
  id: string
  course_id: string
  title: string
  sort: number
  created_at: string
  updated_at: string
}

export interface ChapterRequest {
  title: string
  sort: number
}

export interface Lesson {
  id: string
  chapter_id: string
  title: string
  content_type: number
  content_ref: Record<string, any>
  sort: number
  created_at: string
  updated_at: string
}

export interface LessonRequest {
  title: string
  content_type: number
  content_ref: Record<string, any>
  sort: number
}

export interface CourseOutline {
  course: Course
  chapters: Chapter[]
  lessons: Lesson[]
  progress: Progress[]
}

export interface Progress {
  lesson_id: string
  student_id: string
  status: number
  video_pos: number
  duration_sec: number
  updated_at: string
}

export interface ProgressRequest {
  status: number
  video_pos: number
  duration_sec: number
}

export interface JoinCourseRequest {
  invite_code: string
}

export interface Assignment {
  id: string
  course_id: string
  title: string
  chapter_id?: string
  due_at: string
  max_attempts: number
  late_policy: number
  late_penalty: Record<string, any>
  status: number
  created_at: string
  updated_at: string
}

export interface AssignmentRequest {
  title: string
  chapter_id: string
  due_at: string
  max_attempts: number
  late_policy: number
  late_penalty: Record<string, any>
  items: AssignmentItemInput[]
}

export interface AssignmentItemInput {
  item_code: string
  item_version: string
  score: number
  seq: number
  grading_mode: number
  judger_code: string
}

export interface AssignmentItem {
  id: string
  item_code: string
  item_version: string
  score: number
  seq: number
  grading_mode: number
  judger_code?: string
  title?: string
  type?: number
  difficulty?: number
  body?: Record<string, any>
}

export interface AssignmentDetail {
  assignment: Assignment
  items: AssignmentItem[]
}

export interface Draft {
  assignment_id: string
  student_id: string
  content: Record<string, any>
  updated_at: string
  exists: boolean
}

export interface Submission {
  id: string
  assignment_id: string
  student_id: string
  attempt_no: number
  content: Record<string, any>
  judge_task_ref?: string
  auto_score?: number
  manual_score?: number
  final_score?: number
  comment?: string
  is_late: boolean
  status: number
  submitted_at: string
}

export interface SubmitRequest {
  content_ref: Record<string, any>
}

// ===== M2 Sandbox 模块 =====

export interface SandboxInstance {
  sandbox_id: number
  tenant_id: number
  owner_account_id: number
  runtime_code: string
  runtime_image_version: string
  source_ref: string
  phase: number
  status: number
  tool_access: SandboxToolAccess[]
  resource_usage: SandboxResourceUsage
}

export interface SandboxToolAccess {
  tool_code: string
  kind: number
  endpoint: string
  status: number
}

export interface SandboxCommandToolRunRequest {
  command: string[]
  stdin_base64?: string
  timeout_sec?: number
}

export interface SandboxCommandToolRunResponse {
  stdout_base64: string
  stderr_base64: string
  exit_code: number
}

export interface SandboxResourceUsage {
  cpu_usage_milli: number
  memory_usage_mib: number
  cpu_request_milli: number
  cpu_limit_milli: number
  memory_request_mib: number
  memory_limit_mib: number
  storage_bytes: number
}

export interface SandboxFileReadResponse {
  relative_path: string
  content_base64: string
  content_sha256: string
  content_size: number
}

export interface SandboxFileEntry {
  name: string
  relative_path: string
  is_dir: boolean
  size: number
}

export interface SandboxFileListResponse {
  relative_path: string
  entries: SandboxFileEntry[]
}

export interface SandboxFileWriteRequest {
  relative_path: string
  content_base64: string
}

export interface SandboxFileSaveResponse {
  code_storage_key: string
  code_hash: string
}

export interface SandboxChainRequest {
  payload: Record<string, any>
}

export type SandboxChainResponse = Record<string, any>

// ===== M3 Judge 模块 =====

export interface JudgeTask {
  task_id: string
  tenant_id: string
  source_ref: string
  submitter_id: string
  status: 'queued' | 'judging' | 'done' | 'timeout' | 'failed' | 'error' | 'cancelled'
  existing?: boolean
  result?: JudgeTaskResult
}

export interface JudgeTaskResult {
  passed: boolean
  score: number
  max_score: number
  version?: number
  is_rejudge?: boolean
  details: JudgeResultDetail[]
  snapshot_ref: string
}

export interface JudgeResultDetail {
  case?: string
  source?: string
  target?: string
  passed: boolean
  expected_label?: string
  actual?: string
  hint?: string
}

export interface JudgeManualScoreRequest {
  score: number
  max_score: number
  passed: boolean
  comment: string
}

// ===== M7 Experiment 模块 =====

export interface Experiment {
  id: string
  course_id?: string
  author_id: string
  template_ref?: string
  template_version?: string
  name: string
  description: string
  components: ComponentConfig
  collab_mode: number
  group_config: GroupConfig
  require_report: boolean
  wizard_step: number
  status: number
  created_at: string
  updated_at: string
}

export interface ComponentConfig {
  envs: EnvComponent[]
  sims: SimComponent[]
  checkpoints: CheckpointConfig[]
  stages: StageConfig[]
}

export interface EnvComponent {
  id: string
  runtime_code: string
  runtime_image_version?: string
  tools: string[]
  init_code_ref?: string
  init_script_ref?: string
  keep_alive?: boolean
  snapshot_enabled?: boolean
  keep_alive_minutes?: number
  snapshot_retention_minutes?: number
}

export interface SimComponent {
  id: string
  package_code: string
  version: string
  seed: number
  params: Record<string, any>
}

export interface CheckpointConfig {
  id: string
  judger: string
  item_code: string
  item_version: string
  score: number
  mode?: string
  env_id?: string
  sim_id?: string
  extra_input?: Record<string, any>
}

export interface StageConfig {
  stage: number
  title: string
  description?: string
  components: StageComponents
  unlock_condition?: UnlockCondition
  param_bindings?: ParamBinding[]
}

export interface StageComponents {
  envs?: string[]
  sims?: string[]
}

export interface UnlockCondition {
  type: 'checkpoint' | 'manual'
  checkpoint_id?: string
  min_score?: number
}

export interface ParamBinding {
  target_component: string
  target_param: string
  source_type: 'checkpoint' | 'constant'
  source_ref?: string
  source_path?: string
  constant_value?: any
}

export interface GroupConfig {
  size: number
  roles: string[]
}

export interface ExperimentRequest {
  course_id: number
  template_ref: string
  template_version: string
  name: string
  description: string
  components: ComponentConfig
  collab_mode: number
  group_config: GroupConfig
  require_report: boolean
  wizard_step: number
}

export interface ValidationResult {
  ok: boolean
  issues: ValidationIssue[]
}

export interface ValidationIssue {
  level: string
  message: string
}

export interface CreateInstanceRequest {
  group_id?: number
}

export interface ExperimentInstance {
  instance_id: string
  experiment_id: string
  owner_account_id: string
  group_id?: string
  source_ref: string
  sandboxes: SandboxRef[]
  sims: SimSessionRef[]
  status: number
  score: number
  started_at: string
  finished_at?: string
  last_active_at: string
  checkpoints?: CheckpointResult[]
  stages?: StageState[]
}

export interface SandboxRef {
  component_id: string
  stage: number
  sandbox_id: string
  runtime_code: string
  tools: SandboxTool[]
}

export interface SandboxTool {
  code: string
  kind: number
  endpoint: string
  status: number
}

export interface SimSessionRef {
  component_id: string
  stage: number
  session_id: string
  package_code: string
  version: string
  bundle_ref: string
}

export interface CheckpointResult {
  id: string
  judge_task_ref?: string
  passed: boolean
  score: number
  detail_ref?: string
  binding_output?: Record<string, any>
}

export interface StageState {
  stage: number
  title: string
  description?: string
  status: 'locked' | 'available' | 'active'
  components: StageComponents
  unlock_condition?: UnlockCondition
}

export interface ProgressDTO {
  topic: string
  channel: string
}

export interface CheckpointJudgeRequest {
  code_storage_key?: string
  code_hash?: string
  extra_input?: Record<string, any>
  binding_output?: Record<string, any>
}

export interface ReportDTO {
  id: string
  instance_id: string
  student_id: string
  content_ref: string
  manual_score: number
  comment?: string
  status: number
  submitted_at: string
}

// ===== M8 Contest 模块 =====

export interface Contest {
  id: string
  organizer_id: string
  name: string
  mode: number
  match_mode?: number
  team_mode: number
  signup_start: string
  signup_end: string
  start_at: string
  end_at: string
  freeze_minutes: number
  rules: Record<string, any>
  status: number
  created_at: string
  updated_at: string
}

export interface ContestRequest {
  name: string
  mode: number
  match_mode?: number
  team_mode: number
  signup_start: string
  signup_end: string
  start_at: string
  end_at: string
  freeze_minutes: number
  rules: Record<string, any>
}

export interface ContestProblem {
  id: string
  contest_id: string
  item_code: string
  item_version: string
  score: number
  dynamic_score?: Record<string, any>
  battle_config?: Record<string, any>
  battle_rule?: number
  seq: number
  face?: Record<string, any>
}

export interface ContestProblemRequest {
  item_code: string
  item_version: string
  score: number
  dynamic_score?: Record<string, any>
  battle_config?: Record<string, any>
  battle_rule?: number
  seq: number
}

export interface ContestTeam {
  id: string
  contest_id: string
  name: string
  invite_code?: string
  status: number
  created_at: string
  members: TeamMember[]
}

export interface TeamMember {
  id: string
  team_id: string
  account_id: string
  member_tenant_id: string
  is_leader: boolean
  joined_at: string
}

export interface SignupRequest {
  team_name: string
}

export interface JoinTeamRequest {
  invite_code: string
}

export interface ContestSubmission {
  id: string
  contest_id: string
  problem_id: string
  team_id: string
  submitter_id: string
  content_ref: Record<string, any>
  source_ref: string
  judge_task_ref?: string
  passed: boolean
  score: number
  sandbox_ref?: string
  submitted_at: string
}

export interface ContestSubmitRequest {
  content_ref: Record<string, any>
  code_storage_key?: string
  code_hash?: string
  sandbox_ref?: string
}

export interface EnvRequest {
  runtime_code: string
  runtime_image_version: string
  tool_codes: string[]
  init_code_ref?: string
  init_script_ref?: string
}

export interface EnvSummary {
  sandbox_id: string
  source_ref: string
  status: number
}

export interface BattleEntryRequest {
  problem_id: number
  role: number
  artifact_ref: string
  code_hash: string
}

export interface BattleEntry {
  id: string
  contest_id: string
  problem_id: string
  team_id: string
  role: number
  artifact_ref: string
  code_hash: string
  version_no: number
  is_active: boolean
  submitted_at: string
}

export interface BattleMatch {
  id: string
  contest_id: string
  problem_id: string
  entry_a_id: string
  entry_b_id: string
  source_ref: string
  sandbox_ref?: string
  judge_task_ref?: string
  result?: number
  score_delta: Record<string, any>
  replay_ref?: string
  status: number
  matched_at: string
  finished_at?: string
}

export interface BattleReplayRef {
  match_id: string
  replay_ref: string
}

export interface LadderRank {
  team_id: string
  score: number
  solved_count: number
  last_solve_at?: string
  rank: number
  updated_at: string
}

export interface ResultSnapshot {
  id: string
  tenant_id?: string
  contest_id: string
  final_ranking: Record<string, any>[]
  generated_at: string
}

export interface CheatRecordRequest {
  team_id: number
  type: number
  evidence: Record<string, any>
  action: number
}

export interface CheatRecord {
  id: string
  contest_id: string
  team_id: string
  type: number
  evidence: Record<string, any>
  action: number
  operator_id?: string
  created_at: string
}

export interface CheatSuspect {
  source_ref: string
  submitter_id: string
  score: number
  code_hash?: string
}

export interface ContestRecord {
  contest_id: string
  team_id: string
  score: number
  rank: number
  contest_name: string
  contest_status: number
}

export interface VulnSourceRequest {
  id?: number
  type: number
  name: string
  config: Record<string, any>
  default_level: number
  enabled: boolean
}

export interface VulnSource {
  id: string
  type: number
  name: string
  config: Record<string, any>
  default_level: number
  enabled: boolean
  last_sync_at?: string
}

export interface VulnProblemImportRequest {
  source_id?: number
  external_ref?: string
  title: string
  level: number
  runtime_mode: number
  draft_body: Record<string, any>
}

export interface VulnPrevalidateRequest {
  runtime_code: string
  runtime_image_version: string
  tool_codes: string[]
  init_code_ref?: string
  init_script_ref?: string
}

export interface VulnProblem {
  id: string
  source_id?: string
  external_ref?: string
  title: string
  level: number
  runtime_mode: number
  draft_body: Record<string, any>
  prevalidate_status: number
  prevalidate_detail: Record<string, any>
  content_item_code?: string
  content_item_version?: string
  status: number
}

// ===== M9 Admin 模块 =====

export interface SystemConfig {
  id: string
  scope: number
  tenant_id?: string
  key: string
  value: Record<string, any>
  version: number
  updated_by: string
  updated_at: string
}

export interface ConfigUpdateRequest {
  scope: number
  tenant_id?: string
  value: Record<string, any>
  version: number
  change_log_id?: string
}

export interface ConfigRollbackRequest {
  scope: number
  tenant_id?: string
  version: number
  change_log_id: string
}

export interface ConfigChangeLog {
  id: string
  config_id: string
  tenant_id?: string
  old_value: Record<string, any>
  new_value: Record<string, any>
  operator_id: string
  created_at: string
}

export interface AlertRule {
  id: string
  scope: number
  tenant_id?: string
  name: string
  metric: string
  condition: Record<string, any>
  level: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface AlertRuleRequest {
  scope: number
  tenant_id?: string
  name: string
  metric: string
  condition: Record<string, any>
  level: number
  enabled: boolean
}

export interface AlertEvent {
  id: string
  rule_id: string
  tenant_id?: string
  level: number
  message: string
  status: number
  handler_id?: string
  triggered_at: string
  handled_at?: string
}

export interface AlertEventRequest {
  status: number
}

export interface Statistics {
  scope: number
  tenant_id?: string
  date: string
  metrics: Record<string, any>
}

export interface BackupRecord {
  id: string
  type: number
  size_bytes: number
  status: number
  started_at: string
  finished_at?: string
}

export interface Dashboard {
  scope: number
  tenant_id?: string
  tenant_count?: number
  account_count: number
  teacher_count: number
  student_count: number
  active_account_count: number
  course_count: number
  active_course_count: number
  experiment_count: number
  active_instance_count: number
  contest_count: number
  active_contest_count: number
  active_sandbox_count: number
  pending_apply_count?: number
  resource_quota_snapshot?: Record<string, any>
  generated_at: string
}

export interface MonitoringPanel {
  name: string
  url: string
}

export interface TenantSummary {
  tenant_id: string
  code: string
  name: string
  type: number
  status: number
  deploy_mode: number
  expire_at?: string
  created_at: string
  updated_at: string
}

export interface TenantApplicationSummary {
  application_id: string
  school_name: string
  school_type: number
  contact_name: string
  contact_phone: string
  contact_email: string
  status: number
  submitted_at: string
  reviewed_at?: string
}

export interface AuditLogEntry {
  id: string
  tenant_id: string
  actor_id: string
  actor_role: number
  action: string
  target_type: string
  target_id: string
  detail: string
  ip: string
  trace_id: string
  created_at: string
}

export interface AuditQueryParams {
  actor_id?: string
  action?: string
  target_type?: string
  from?: string
  to?: string
  page?: number
  size?: number
}

export interface AuditQueryResult {
  list: AuditLogEntry[]
  total: number
  page: number
  size: number
}

export interface AuditExportTask {
  task_id: string
  channel: string
  subject: string
  status: string
  content_type?: string
  file_name?: string
  attempt_count: number
  max_attempts: number
  artifact_size?: number
  artifact_content_type?: string
  artifact_file_name?: string
  created_at: string
  updated_at: string
  completed_at?: string
  next_attempt_after?: string
}

// ===== M10 Notify 模块 =====

export interface Notification {
  id: string
  type: string
  title: string
  content: string
  link?: string
  is_read: boolean
  read_at?: string
  created_at: string
}

export interface NotificationPreference {
  type: string
  enabled: boolean
}

export interface Announcement {
  id: string
  tenant_id?: string
  title: string
  content: string
  scope: number
  target_roles?: number[]
  publisher_id: string
  published_at: string
  expire_at?: string
  is_read: boolean
}

export interface AnnouncementRequest {
  title: string
  content: string
  scope: number
  target_roles: number[]
  expire_at?: string
}

// ===== M11 Grade 模块 =====

export interface LevelRule {
  min: number
  grade: string
  gpa: number
}

export interface WarningRules {
  fail_count: number
  min_gpa: number
}

export interface LevelConfig {
  id: string
  tenant_id: string
  name: string
  mapping: LevelRule[]
  warning_rules: WarningRules
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface LevelConfigRequest {
  name: string
  mapping: LevelRule[]
  warning_rules: WarningRules
  is_default: boolean
}

export interface Semester {
  id: string
  tenant_id: string
  name: string
  start_date: string
  end_date: string
  is_current: boolean
}

export interface SemesterRequest {
  name: string
  start_date: string
  end_date: string
  is_current: boolean
}

export interface GradeReview {
  id: string
  tenant_id: string
  course_id: string
  semester_id?: string
  submitter_id: string
  reviewer_id?: string
  status: number
  is_locked: boolean
  comment?: string
  submitted_at: string
  reviewed_at?: string
}

export interface GradeReviewRequest {
  course_id: string
  semester_id?: string
  comment?: string
}

export interface ReviewDecision {
  semester_id?: string
  comment?: string
}

export interface CourseGrade {
  course_id: string
  student_id: string
  final_total: number
  credits: number
}

export interface GradeSummary {
  student_id: string
  semester_id?: string
  total_credits: number
  gpa: number
  cumulative_gpa: number
  course_grades: CourseGrade[]
  computed_at: string
}

export interface GradeAppeal {
  id: string
  tenant_id: string
  student_id: string
  course_id: string
  reason: string
  status: number
  handler_id?: string
  result_comment?: string
  created_at: string
  handled_at?: string
}

export interface GradeAppealRequest {
  course_id: string
  reason: string
}

export interface GradeWarning {
  id: string
  tenant_id: string
  student_id: string
  semester_id: string
  type: number
  detail: Record<string, any>
  status: number
  created_at: string
}

export interface WarningScanResult {
  scanned: number
  created: number
}

export interface GradeTranscript {
  id: string
  tenant_id: string
  student_id: string
  scope: number
  semester_id?: string
  generated_at: string
}

export interface TranscriptRequest {
  student_id?: string
  scope: number
  semester_id?: string
}

export interface TranscriptDownloadGrant {
  token: string
  transcript: GradeTranscript
  expires_at: string
}

// ===== M4 Sim 模块 =====

export interface SimPackageMeta {
  id: string
  code: string
  version: string
  name: string
  category: string
  compute: 'frontend' | 'backend'
  scale_limit?: Record<string, any>
  bundle_hash?: string
  backend_adapter?: string
  backend_config?: Record<string, any>
  status: 'draft' | 'reviewing' | 'published' | 'archived' | 'rejected'
  created_at: string
  updated_at: string
}

export interface SimPackageSubmit {
  bundle: File
  code: string
  version: string
  name: string
  category: string
  compute: 'frontend' | 'backend'
  scale_limit?: Record<string, any>
  backend_adapter?: string
  backend_config?: Record<string, any>
}

export interface SimBundleDownloadGrant {
  token: string
  bundle_hash: string
  expires_at: string
}

export interface SimPackageSubmissionResult extends SimPackageMeta {
  review: SimPackageReview
}

export interface SimReviewDecision {
  package: SimPackageMeta
  review: SimPackageReview
}

export interface SimValidationStatus {
  status?: string
  message?: string
}

export interface SimStaticScanReport {
  status?: string
  findings?: string[]
}

export interface SimValidationReport {
  bundle_hash?: string
  metadata_validation?: SimValidationStatus
  static_scan?: SimStaticScanReport
  determinism_check?: SimValidationStatus
  worker_preview?: SimValidationStatus
  details?: Record<string, string>
}

export interface SimValidationReportRequest {
  determinism_check: SimValidationStatus
  worker_preview: SimValidationStatus
  details: Record<string, string>
}

export interface SimPackageReview {
  id: string
  package_id: string
  submitter_id: string
  preview_report: SimValidationReport
  reviewer_id?: string
  result: 'pending' | 'approved' | 'rejected'
  comment?: string
  created_at: string
  updated_at?: string
  package?: {
    code: string
    version: string
    name: string
    category: string
    compute: 'frontend' | 'backend'
    status: 'draft' | 'reviewing' | 'published' | 'archived' | 'rejected'
  }
}

export interface SimActionLog {
  seq: number
  at_tick: number
  event_type: string
  payload: Record<string, any>
  created_at?: string
}

export interface SimReplay {
  package_code: string
  version: string
  seed: number
  init_params: Record<string, any>
  actions: SimActionLog[]
}

export interface SimShareCreate {
  expire_at?: string
}

export interface SimShareResult {
  code: string
  expire_at?: string
  status: 'active' | string
}

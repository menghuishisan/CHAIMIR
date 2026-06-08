// Package teaching 实现 M6 教学(第2层 业务)。
// 职责:课程/作业/进度/单课程成绩;判题调 M3,订阅 judge.completed。
// 边界:不存题目内容(在 content)、不算跨课程 GPA(在 grade)。
package teaching

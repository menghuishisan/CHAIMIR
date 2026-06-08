// Package grade 实现 M11 成绩中心(第3层 聚合)。
// 职责:跨课程聚合/GPA/审核/申诉(只读 teaching 的 /grades/internal)。
// 边界:不算单课程成绩(在 teaching);只读不跨写。
package grade

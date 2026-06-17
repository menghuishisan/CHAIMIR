#!/usr/bin/env sh
# 本脚本把公开初始化素材复制到学生工作区。
set -eu

SOURCE_DIR="${CHAIMIR_SEED_SOURCE:-/seed-source}"
TARGET_DIR="${CHAIMIR_SEED_TARGET:-/workspace}"

if [ ! -d "$SOURCE_DIR" ]; then
  echo "seed source not found: $SOURCE_DIR" >&2
  exit 64
fi

case "$TARGET_DIR" in
  /workspace|/workspace/*) ;;
  *)
    echo "seed target must be under /workspace" >&2
    exit 66
    ;;
esac

if find "$SOURCE_DIR" -type l | grep -q .; then
  echo "seed source contains symlink" >&2
  exit 65
fi

mkdir -p "$TARGET_DIR"

# 只复制平台挂载进来的普通素材目录,不读取任何 Secret、符号链接或控制面路径。
cp -R "$SOURCE_DIR"/. "$TARGET_DIR"/

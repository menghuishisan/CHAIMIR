#!/usr/bin/env sh
# 本脚本把公开初始化素材复制到学生工作区。
set -eu

: "${CHAIMIR_SEED_SOURCE:?CHAIMIR_SEED_SOURCE is required}"
: "${CHAIMIR_SEED_TARGET:?CHAIMIR_SEED_TARGET is required}"
SOURCE_DIR=${CHAIMIR_SEED_SOURCE}
TARGET_DIR=${CHAIMIR_SEED_TARGET}

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

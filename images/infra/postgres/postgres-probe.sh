#!/bin/sh
# 本脚本使用运行期数据库身份探测 PostgreSQL。
set -eu
: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"
exec pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -h 127.0.0.1 -p 5432

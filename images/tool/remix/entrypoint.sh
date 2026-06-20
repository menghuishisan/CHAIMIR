#!/bin/sh
# 本脚本在只读根文件系统下创建 Remix nginx 运行期临时目录。
set -eu

mkdir -p /tmp/nginx/client_temp /tmp/nginx/proxy_temp /tmp/nginx/fastcgi_temp /tmp/nginx/uwsgi_temp /tmp/nginx/scgi_temp
exec nginx -g 'daemon off;'

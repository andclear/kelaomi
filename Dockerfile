# ---- 第一阶段：构建 ----
# 使用与项目匹配的 Go 版本
FROM golang:1.24-alpine AS builder

# 为 CGO 安装构建依赖 (gcc, musl-dev等)
RUN apk add --no-cache build-base

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制所有源代码
COPY . .

# 构建 Go 应用，启用 CGO 以支持 go-sqlite3
# 注意：已移除 CGO_ENABLED=0
RUN go build -ldflags="-w -s" -o /atlassian-proxy .

# ---- 第二阶段：运行 ----
# 使用一个非常小的基础镜像
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /atlassian-proxy /app/atlassian-proxy

# 复制 Web 界面所需的 templates 和 static 目录
COPY --from=builder /app/templates /app/templates/
COPY --from=builder /app/static /app/static/

# 创建非 root 用户和数据目录
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && \
    mkdir -p /data && \
    chown appuser:appgroup /data

# 声明数据卷以实现持久化
VOLUME /data

# 切换到非 root 用户
USER appuser

# 暴露应用端口
EXPOSE 8000

# 启动应用
CMD ["./atlassian-proxy"]

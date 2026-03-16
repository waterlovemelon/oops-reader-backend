# 部署说明

## 快速启动（推荐）

服务已成功启动并测试。

### 当前运行状态
- **进程 ID**: 7827
- **端口**: 8080
- **日志**: /tmp/oops-server.log

### 已测试接口

#### 1. 健康检查 ✅
```bash
curl http://localhost:8080/health
```

响应：
```json
{
  "message": "service is healthy",
  "status": "ok"
}
```

#### 2. 书籍解析 - 中文书籍 ✅
```bash
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "三体"}'
```

成功从 Google Books 检索到：
- 书名: Trekropparsproblemet（三体的挪威译本）
- 作者: Liu Cixin
- 封面: http://books.google.com/books/content?id=...
- 评分: 4.5/5

#### 3. 书籍解析 - 带作者 ✅
```bash
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "1984/乔治·奥威尔"}'
```

成功解析：
- 书名: 1984
- 作者: 乔治·奥威尔 / George Orwell
- 来源: Google Books

#### 4. 书籍封面获取 ✅
```bash
curl "http://localhost:8080/v1/utils/book-cover?book_name=三体"
```

成功获取封面：
- 从百度图片获取
- 去除水印
- 封面URL: https://lib.gzccc.edu.cn/__local/...

### 重新启动服务

如果需要重启服务：

#### 1. 停止当前服务
```bash
kill 7827
# 或
pkill -f "bin/simple"
```

#### 2. 重新构建
```bash
cd /Users/jason/Workspace/Code/oops/reader/oops-reader-backend
go build -o bin/simple ./cmd/simple
```

#### 3. 启动服务
```bash
./bin/simple > /tmp/oops-server.log 2>&1 &
```

#### 4. 验证启动
```bash
sleep 2
curl http://localhost:8080/health
```

### 测试所有接口

```bash
# 运行测试脚本
./test_api.sh
```

### 部署到生产环境

#### 1. 创建 systemd 服务（Linux）

创建 `/etc/systemd/system/oops-reader.service`:

```ini
[Unit]
Description=Oops Reader Backend
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/oops-reader-backend
ExecStart=/opt/oops-reader-backend/bin/simple
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启动服务：
```bash
sudo systemctl daemon-reload
sudo systemctl enable oops-reader
sudo systemctl start oops-reader
```

#### 2. 使用 Docker（可选）

创建 `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o bin/simple ./cmd/simple

FROM alpine:3.19
RUN apk add --no-cache python3 ca-certificates
WORKDIR /opt/app
COPY --from=builder /app/bin/simple .
COPY --from=builder /app/utils ./utils
EXPOSE 8080
CMD ["./bin/simple"]
```

构建和运行：
```bash
docker build -t oops-reader-backend .
docker run -d -p 8080:8080 --name oops-reader oops-reader-backend
```

#### 3. 使用 Nginx 反向代理

```nginx
location / {
    proxy_pass http://localhost:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

### 监控和日志

查看日志：
```bash
tail -f /tmp/oops-server.log
```

检查进程：
```bash
ps aux | grep oops-server
```

### 性能优化建议

1. **启用 gzip 压缩** (需要使用 Gin 版本)
2. **添加 Redis 缓存** 热门书籍信息
3. **使用 CDN** 存储封面图片
4. **配置数据库连接池** (完整版)
5. **添加限流** 防止滥用

### 故障排查

#### 服务无法启动
```bash
# 检查端口是否被占用
lsof -i:8080

# 检查日志
cat /tmp/oops-server.log

# 手动运行查看错误
./bin/simple
```

#### Python 脚本失败
```bash
# 检查 Python 版本
python3 --version

# 测试脚本
cd utils
python3 book_parser.py "测试书名"
python3 bookCover.py "测试书名"
```

#### 网络请求超时
- 豆瓣/Google Books 有访问限制
- 考虑添加重试机制
- 使用代理转发请求

## 测试报告

### 测试日期
2025-03-16

### 测试环境
- OS: macOS
- Go: 1.26.1
- Python: 3.11.4
- 服务: bin/simple (标准库版本)

### 测试结果

| 接口 | 状态 | 响应时间 | 备注 |
|-----|------|---------|------|
| /health | ✅ 通过 | <10ms | 正常 |
| /v1/utils/parse-book-info (三体) | ✅ 通过 | ~15s | 从Google Books获取 |
| /v1/utils/parse-book-info (1984/作者) | ✅ 通过 | ~10s | 正确解析作者 |
| /v1/utils/book-cover (三体) | ✅ 通过 | ~8s | 百度来源，去水印 |
| /v1/utils/book-cover (1984) | ✅ 通过 | ~7s | 成功获取封面 |

### 已知问题

1. **Google Books 限流**: 频繁请求可能被限制
2. **豆瓣 API**: 部分情况下无法获取数据
3. **网络延迟**: 跨国请求较慢
4. **封面来源**: 有些书籍封面无法获取

### 改进建议

1. 添加本地数据库缓存已搜索的书籍
2. 实现异步任务队列
3. 添加用户认证和权限控制
4. 实现完整的用户书架和阅读进度功能

## 联系支持

如有问题，请查看：
- 项目文档: `README.md`
- 架构设计: `docs/backend-architecture-and-db-design.md`
- 工具说明: `utils/README.md`

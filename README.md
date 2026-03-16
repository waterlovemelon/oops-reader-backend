# Oops Reader Backend

基于 Golang 的阅读后端服务，支持用户阅读数据同步、书籍信息检索等功能。

## 已实现功能

### 数据库
- 完整的 MySQL 数据库设计，包含 19 个表：
  - 用户认证：users, user_auth_providers, user_sessions
  - 书籍主数据：books, authors, book_authors, book_aliases, book_external_sources
  - 用户阅读数据：user_bookshelves, reading_progress, reading_sessions, reading_daily_stats, user_notes, user_preferences
  - 同步与规范化：sync_operations, sync_cursors, book_title_normalization_tasks, book_search_logs

### API 接口

#### 1. 书籍信息解析
- **接口**: `POST /v1/utils/parse-book-info`
- **功能**: 解析书籍名称和作者，联网检索书籍信息
- **请求体**:
```json
{
  "input": "三体"
}
```
- **响应**:
```json
{
  "input": "三体",
  "parsed_title": "三体",
  "parsed_author": null,
  "search_results": [
    {
      "source": "douban",
      "title": "三体",
      "author": null,
      "found": false
    },
    {
      "source": "google_books",
      "title": "The Three-Body Problem",
      "author": "Liu Cixin",
      "isbn": "0765382032",
      "url": "...",
      "cover": "...",
      "rating": 4.5,
      "found": true
    }
  ],
  "best_match": { ... }
}
```

#### 2. 获取书籍封面
- **接口**: `GET /v1/utils/book-cover?book_name=书名`
- **功能**: 根据书名获取书籍封面
- **响应**:
```json
{
  "source": "douban" | "google" | "none",
  "title": "书籍标题",
  "author": "作者名",
  "imageUrl": "封面图片URL"
}
```

#### 3. 健康检查
- **接口**: `GET /health`
- **功能**: 检查服务健康状态和数据库连接

## 配置文件管理

### .gitignore

项目使用 `.gitignore` 文件排除不应该提交到版本控制的文件：

**被忽略的文件类型:**
- 编译产物: `bin/`, `*.exe`, `*.dll`
- 配置文件: `config.yaml` (包含敏感信息)
- 依赖缓存: `vendor/`, `__pycache__/`, `go.sum`
- 日志文件: `*.log`, `logs/`
- IDE 配置: `.vscode/`, `.idea/`, `*.swp`
- Python 虚拟环境: `venv/`, `.venv`
- 敏感信息: `.env`, `credentials.json`, `*.pem`, `*.key`

**首次部署:**
```bash
# 从示例配置创建实际配置
cp config.yaml.example config.yaml

# 编辑配置文件
nano config.yaml

# 验证配置（可选）
./scripts/verify-gitignore.sh
```

**相关文档:**
- [Git Ignore 指南](./docs/gitignore-guide.md) - 详细说明和最佳实践
- [配置文件指南](./docs/config-guide.md) - 配置项详细说明
- [配置总结](./docs/gitignore-summary.md) - 配置完成总结

### 配置文件

**config.yaml** (不提交到 Git)
```yaml
database:
  host: localhost
  port: 3306
  user: root
  password: "your_password_here"  # 必须修改
  database: oops_reader

jwt:
  secret: "change-this-in-production"  # 必须修改
```

**生成安全密钥:**
```bash
# OpenSSL
openssl rand -base64 32

# 或使用 Go
go run -e 'import "crypto/rand"; import "encoding/base64"; b := make([]byte, 32); rand.Read(b); println(base64.StdEncoding.EncodeToString(b))'
```

详细配置说明: [docs/config-guide.md](./docs/config-guide.md)

## 项目结构

```
oops-reader-backend/
├── cmd/
│   ├── api/          # HTTP 服务器
│   └── worker/       # 后台任务处理
├── internal/
│   ├── platform/     # 基础设施层
│   │   ├── config/   # 配置管理
│   │   ├── db/       # 数据库
│   │   ├── log/      # 日志
│   │   └── ...
│   ├── transport/    # 传输层
│   │   └── http/     # HTTP 处理
│   └── ...           # 业务领域
├── migrations/       # 数据库迁移文件
├── utils/            # Python 工具脚本
│   ├── book_parser.py      # 书籍信息解析
│   ├── bookCover.py        # 书籍封面获取
│   └── ...
├── go.mod
└── config.yaml
```

## 部署步骤

### 1. 安装 Go 1.21+

```bash
# macOS
brew install go

# Linux
# 下载 https://go.dev/dl/ 并安装
```

### 2. 配置数据库

```bash
# 创建数据库
mysql -u root -p
CREATE DATABASE oops_reader CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

# 运行迁移
mysql -u root -p oops_reader < migrations/001_init_user_auth.sql
mysql -u root -p oops_reader < migrations/002_init_books.sql
mysql -u root -p oops_reader < migrations/003_init_user_reading_data.sql
mysql -u root -p oops_reader < migrations/004_init_sync_and_normalization.sql
```

### 3. 配置环境

编辑 `config.yaml`:

```yaml
database:
  host: localhost
  port: 3306
  user: root
  password: "your_password"
  database: oops_reader

jwt:
  secret: "your-secret-key-change-in-production"
```

### 4. 启动服务

#### 方式一：简化版（无需安装Gin等依赖）

```bash
# 构建
go build -o bin/simple ./cmd/simple

# 启动服务
./bin/simple
```

服务将监听 `http://localhost:8080`

#### 方式二：完整版（需要Gin框架）

```bash
# 配置国内代理（如果下载慢）
export GOPROXY=https://goproxy.cn,direct

# 下载依赖
go mod download

# 构建
go build -o bin/api ./cmd/api

# 启动服务
./bin/api
```

## 测试接口

### 1. 健康检查
```bash
curl http://localhost:8080/health
```

### 2. 解析书籍信息
```bash
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "三体"}'
```

### 3. 获取书籍封面
```bash
curl "http://localhost:8080/v1/utils/book-cover?book_name=三体"
```

## 技术栈

- **HTTP 服务**: Gin
- **数据库**: MySQL + go-sql-driver
- **日志**: Zap
- **配置**: Viper
- **认证**: JWT (golang-jwt/jwt)
- **书籍检索**: Python 脚本 + 豆瓣/Google Books API

## 当前服务状态

✅ **服务已部署并运行中**

- **运行中**: 是
- **进程 ID**: 7827
- **监听端口**: 8080
- **访问地址**: http://localhost:8080
- **日志文件**: /tmp/oops-server.log

### 实时测试

```bash
# 健康检查
curl http://localhost:8080/health
# 预期: {"message":"service is healthy","status":"ok"}

# 解析书籍（中文）
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "三体"}'

# 获取封面
curl "http://localhost:8080/v1/utils/book-cover?book_name=三体"
```

详细部署文档请查看: `DEPLOYMENT.md`

## 当前状态

✅ 项目结构完整
✅ 数据库设计完成（19个表）
✅ 基础 HTTP 服务器框架
✅ 书籍解析 API
✅ 书籍封面 API
⏳ 其他业务逻辑（待实现）
⏳ 用户认证系统（待实现）
⏳ 数据同步功能（待实现）

## 注意事项

1. **Go 版本**: 项目需要 Go 1.21 或更高版本
2. **Python 依赖**: 需要 Python 3.7+ 和 requests 库
3. **网络访问**: 书籍信息检索依赖互联网连接
4. **API 配额**: Google Books API 有请求限制
5. **生产环境**: 需修改 JWT secret 和数据库密码

## 开发计划

- [ ] 实现用户注册/登录
- [ ] 实现书架管理
- [ ] 实现阅读进度同步
- [ ] 实现读书笔记
- [ ] 添加 Redis 缓存
- [ ] 实现多设备增量同步

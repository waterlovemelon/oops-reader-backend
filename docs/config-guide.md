# 配置文件说明

本项目的配置文件不应提交到版本控制中，因为它们包含敏感信息（如数据库密码、API密钥等）。

## 配置文件使用

### 1. 初始化配置

首次部署时，从示例配置复制：

```bash
cp config.yaml.example config.yaml

# 编辑配置文件
nano config.yaml
```

### 2. 配置文件结构

`config.yaml` 包含以下部分：

```yaml
server:
  # 服务器配置
  port: 8080
  mode: debug  # debug/release
  read_timeout: 30s
  write_timeout: 30s

database:
  # 数据库配置
  host: localhost
  port: 3306
  user: root
  password: your_database_password
  database: oops_reader
  max_open_conns: 100
  max_idle_conns: 10

jwt:
  # JWT 配置
  secret: your-secret-key-change-in-production
  access_token_expiry: 1h
  refresh_token_expiry: 168h  # 7天

log:
  # 日志配置
  level: info  # debug/info/warn/error
  format: console  # console/json
  output_path: stdout  # stdout 或文件路径
```

### 3. 配置说明

#### server 部分

| 参数 | 说明 | 默认值 | 备注 |
|------|------|--------|------|
| port | 服务监听端口 | 8080 | 需要在防火墙开放 |
| mode | 运行模式 | debug | 生产环境使用 release |
| read_timeout | 读取超时 | 30s | 单个请求的最大读取时间 |
| write_timeout | 写入超时 | 30s | 单个请求的最大写入时间 |

#### database 部分

| 参数 | 说明 | 示例 |
|------|------|------|
| host | 数据库主机 | localhost 或 IP 地址 |
| port | 数据库端口 | 3306 (MySQL 默认) |
| user | 数据库用户名 | oops_reader (推荐专用用户) |
| password | 数据库密码 | 强密码（建议16位以上） |
| database | 数据库名 | oops_reader |

#### jwt 部分

| 参数 | 说明 | 建议 |
|------|------|------|
| secret | JWT 签名密钥 | 至少32个字符，生产环境必须修改 |
| access_token_expiry | 访问令牌有效期 | 1h (1小时) |
| refresh_token_expiry | 刷新令牌有效期 | 168h (7天) |

#### log 部分

| 参数 | 说明 | 可选值 |
|------|------|--------|
| level | 日志级别 | debug/info/warn/error |
| format | 日志格式 | console/json |
| output_path | 输出路径 | stdout 或日志文件路径 |

### 4. 生产环境配置建议

#### 安全配置

```yaml
server:
  mode: release  # 生产模式

database:
  user: oops_reader  # 使用专用用户，而非 root
  password: "strong_rand0m_p@ssw0rd_here"

jwt:
  secret: "generate-using-openssl-rand-base64-32"
  access_token_expiry: 30m  # 减少有效期
```

生成安全的 JWT 密钥：

```bash
# 使用 OpenSSL 生成
openssl rand -base64 32

# 或使用 Go
go run -e 'import "crypto/rand"; import "encoding/base64"; b := make([]byte, 32); rand.Read(b); println(base64.StdEncoding.EncodeToString(b))'
```

#### 性能优化

```yaml
server:
  read_timeout: 10s
  write_timeout: 60s  # 书籍解析可能需要较长时间

database:
  max_open_conns: 200
  max_idle_conns: 20
  conn_max_lifetime: 2h
  conn_max_idle_time: 30m
```

#### 日志配置

```yaml
log:
  level: info  # 生产环境使用 info
  format: json  # 便于日志分析
  output_path: /var/log/oops-reader-backend/app.log
```

### 5. 环境变量

除了配置文件，也支持通过环境变量配置：

```bash
# 服务器配置
export OOPS_SERVER_PORT=8080
export OOPS_SERVER_MODE=release

# 数据库配置
export OOPS_DB_HOST=localhost
export OOPS_DB_PORT=3306
export OOPS_DB_USER=oops_reader
export OOPS_DB_PASS=your_password
export OOPS_DB_NAME=oops_reader

# JWT 配置
export OOPS_JWT_SECRET=your-secret-key
```

环境变量的优先级高于配置文件。

### 6. 配置验证

修改配置后，建议先进行测试：

```bash
# 停止服务
sudo systemctl stop oops-reader

# 启动服务
sudo systemctl start oops-reader

# 查看日志
sudo journalctl -u oops-reader -f

# 测试接口
curl http://localhost:8080/health
```

### 7. 常见问题

#### 配置文件格式错误

症状：服务无法启动

检查 YAML 格式：
```bash
# 使用 Python 检查 YAML
python3 -c "import yaml; yaml.safe_load(open('config.yaml'))"

# 使用在线验证工具
# https://www.yamllint.com/
```

#### 数据库连接失败

症状：日志显示 "Failed to connect to database"

检查事项：
1. MySQL 服务是否运行
2. 用户名密码是否正确
3. 数据库是否已创建
4. 防火墙是否开放 3306 端口

```bash
# 测试连接
mariadb -u oops_reader -p'your_password' oops_reader
```

#### 端口被占用

症状：服务启动失败，提示端口已使用

查看占用进程：
```bash
sudo lsof -i:8080
```

修改配置文件中的端口或终止占用进程。

### 8. 多环境配置

建议为不同环境创建不同的配置文件：

```
config/
├── development.yaml      # 开发环境
├── staging.yaml          # 测试环境
└── production.yaml       # 生产环境
```

启动时指定配置文件：

```bash
export OOPS_CONFIG_FILE=/path/to/config/production.yaml
./bin/simple
```

### 9. 敏感信息处理

**不要提交到版本控制：**
- ✅ database.password
- ✅ jwt.secret
- ✅ 任何 API 密钥
- ✅ 第三方凭证

**安全的做法：**
1. 使用环境变量
2. 使用密钥管理系统（Vault、AWS Secrets Manager）
3. 配置文件添加到 `.gitignore`

### 10. 配置更新

更新配置后：

```bash
# 重新加载配置（如果支持热重载）
sudo systemctl restart oops-reader

# 验证配置
curl http://localhost:8080/health
```

---

**相关文档：**
- [部署文档](./linux-deployment.md)
- [API 文档](./api.md)
- [架构设计](./backend-architecture-and-db-design.md)

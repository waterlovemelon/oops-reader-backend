# Linux 部署指南

本文档提供在 Linux 环境上部署 Oops Reader Backend 的完整指南。

---

## 目录

- [系统要求](#系统要求)
- [快速部署](#快速部署)
- [详细部署步骤](#详细部署步骤)
  - [1. 系统准备](#1-系统准备)
  - [2. 安装依赖](#2-安装依赖)
  - [3. 部署项目](#3-部署项目)
  - [4. 数据库配置](#4-数据库配置)
  - [5. 服务配置](#5-服务配置)
  - [6. 启动服务](#6-启动服务)
- [Docker 部署](#docker-部署)
- [Nginx 反向代理](#nginx-反向代理)
- [监控和维护](#监控和维护)
- [故障排查](#故障排查)

---

## 系统要求

### 最低要求
- **操作系统**: Ubuntu 22.04+ / Debian 12+ / CentOS 8+
- **CPU**: 1核心
- **内存**: 512MB
- **磁盘**: 1GB

### 推荐配置
- **操作系统**: Debian 13 / Ubuntu 22.04 LTS
- **CPU**: 2核心
- **内存**: 2GB
- **磁盘**: 10GB

---

## 快速部署

### Debian 13 / Ubuntu 22.04+

```bash
# 1. 安装依赖
sudo apt update
sudo apt install -y git golang-1.21 python3 python3-venv mariadb-server

# 2. 克隆项目（或上传代码）
cd /opt
sudo git clone https://github.com/oops-reader/oops-reader-backend.git
cd oops-reader-backend

# 3. 创建 Python 虚拟环境并安装依赖
cd /opt/oops-reader-backend
python3 -m venv .venv
source .venv/bin/activate
pip install requests

# 4. 配置数据库
sudo mariadb -e "CREATE DATABASE oops_reader CHARACTER SET utf8mb4;"
sudo mariadb -e "CREATE USER 'oops_reader'@'localhost' IDENTIFIED BY 'strong_password';"
sudo mariadb -e "GRANT ALL PRIVILEGES ON oops_reader.* TO 'oops_reader'@'localhost';"
sudo mariadb -e "FLUSH PRIVILEGES;"
sudo mariadb oops_reader < migrations/001_init_user_auth.sql
sudo mariadb oops_reader < migrations/002_init_books.sql
sudo mariadb oops_reader < migrations/003_init_user_reading_data.sql
sudo mariadb oops_reader < migrations/004_init_sync_and_normalization.sql

# 5. 构建并运行
go build -o bin/simple ./cmd/simple
./bin/simple > /var/log/oops-reader-backend/app.log 2>&1 &
```

### CentOS/RHEL

```bash
# 1. 安装依赖
sudo yum update -y
sudo yum install -y git golang python3 python3-venv mariadb-server

# 2. 克隆项目
cd /opt
sudo git clone https://github.com/oops-reader/oops-reader-backend.git
cd oops-reader-backend

# 3. 创建 Python 虚拟环境并安装依赖
python3 -m venv .venv
source .venv/bin/activate
pip install requests

# 4. 配置数据库
sudo systemctl start mariadb
sudo mariadb -e "CREATE DATABASE oops_reader CHARACTER SET utf8mb4;"
sudo mariadb -e "CREATE USER 'oops_reader'@'localhost' IDENTIFIED BY 'strong_password';"
sudo mariadb -e "GRANT ALL PRIVILEGES ON oops_reader.* TO 'oops_reader'@'localhost';"
sudo mariadb -e "FLUSH PRIVILEGES;"
sudo mariadb oops_reader < migrations/001_init_user_auth.sql
sudo mariadb oops_reader < migrations/002_init_books.sql
sudo mariadb oops_reader < migrations/003_init_user_reading_data.sql
sudo mariadb oops_reader < migrations/004_init_sync_and_normalization.sql

# 5. 构建并运行
go build -o bin/simple ./cmd/simple
./bin/simple
```

---

## 详细部署步骤

### 1. 系统准备

#### 1.1 更新系统

**Debian/Ubuntu:**
```bash
sudo apt update
sudo apt upgrade -y
```

**CentOS/RHEL:**
```bash
sudo yum update -y
```

#### 1.2 创建部署用户（可选，但推荐）

```bash
# 创建专用用户
sudo useradd -r -s /bin/false oops-reader
sudo mkdir -p /opt/oops-reader-backend
sudo chown -R oops-reader:oops-reader /opt/oops-reader-backend
```

---

### 2. 安装依赖

#### 2.1 安装 Git

**Debian/Ubuntu:**
```bash
sudo apt install -y git
```

**CentOS/RHEL:**
```bash
sudo yum install -y git
```

#### 2.2 安装 Go 1.21+

**Debian/Ubuntu:**
```bash
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

# 配置环境变量
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile
source /etc/profile

# 验证安装
go version
```

**CentOS/RHEL:**
```bash
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile
source /etc/profile

go version
```

#### 2.3 安装 Python 3 和虚拟环境

**Debian/Ubuntu:**
```bash
sudo apt install -y python3 python3-venv

# 验证安装
python3 --version
python3 -m venv --help
```

**CentOS/RHEL:**
```bash
sudo yum install -y python3 python3-venv

# 验证安装
python3 --version
python3 -m venv --help
```

**注意**: Debian 13+ 不支持直接使用 pip3 安装系统包，必须使用虚拟环境。

#### 2.4 安装 MariaDB

**Debian/Ubuntu:**
```bash
sudo apt install -y mariadb-server

# 启动 MariaDB
sudo systemctl start mariadb
sudo systemctl enable mariadb

# Debian 13 特殊说明：系统使用 unix_socket 认证，直接 root 用户即可登录
# 推荐创建应用专用用户（如后面步骤所示）
# 如需设置 root 密码，登录后执行：ALTER USER 'root'@'localhost IDENTIFIED BY 'password';
```

**CentOS/RHEL:**
```bash
sudo yum install -y mariadb-server

# 启动 MariaDB
sudo systemctl start mariadb
sudo systemctl enable mariadb
```

#### 2.5 配置 MariaDB Root 密码（仅首次）

```bash
# 登录 MariaDB（首次可能没有密码或使用系统 sudo）
sudo mariadb
```

```sql
-- 设置 root 密码（根据 Debian 11+ 的安全策略）
-- Debian 13 可能使用 unix_socket 认证
ALTER USER 'root'@'localhost IDENTIFIED BY 'your_root_password';
FLUSH PRIVILEGES;

-- 或者创建应用的专用数据库和用户，无需修改 root 密码
CREATE DATABASE oops_reader CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'oops_reader'@'localhost' IDENTIFIED BY 'strong_password_here';
GRANT ALL PRIVILEGES ON oops_reader.* TO 'oops_reader'@'localhost;
FLUSH PRIVILEGES;

EXIT;
```

---

### 3. 部署项目

#### 3.1 获取项目代码

**方式一：Git 克隆**
```bash
cd /opt
sudo git clone https://github.com/oops-reader/oops-reader-backend.git
cd oops-reader-backend

# 设置权限
sudo chown -R oops-reader:oops-reader /opt/oops-reader-backend
```

**方式二：上传代码**
```bash
# 在本地打包项目
tar czf oops-reader-backend.tar.gz oops-reader-backend/

# 上传到服务器
scp oops-reader-backend.tar.gz user@server:/opt/

# 在服务器上解压
cd /opt
sudo tar xzf oops-reader-backend.tar.gz
sudo chown -R oops-reader:oops-reader /opt/oops-reader-backend
```

#### 3.2 创建 Python 虚拟环境并安装依赖

```bash
cd /opt/oops-reader-backend

# 创建项目级虚拟环境
python3 -m venv .venv

# 激活虚拟环境
source .venv/bin/activate

# 安装依赖
pip install --upgrade pip
pip install requests

# 创建依赖文件（可选）
pip freeze > requirements.txt

# 测试 Python 脚本
cd utils
python3 book_parser.py "测试"

# 退出虚拟环境
deactivate

# 返回项目根目录
cd /opt/oops-reader-backend
```

---

### 4. 数据库配置

#### 4.1 创建数据库和用户

```bash
# 使用 sudo 或 root 登录
sudo mariadb
```

```sql
-- 创建数据库
CREATE DATABASE oops_reader CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 创建专用用户（推荐）
CREATE USER 'oops_reader'@'localhost' IDENTIFIED BY 'your_strong_password_here';

-- 授权
GRANT ALL PRIVILEGES ON oops_reader.* TO 'oops_reader'@'localhost';
FLUSH PRIVILEGES;

EXIT;
```

#### 4.2 导入数据库结构

```bash
cd /opt/oops-reader-backend

# 导入迁移文件（使用专用用户）
mariadb -u oops_reader -p'strong_password' oops_reader < migrations/001_init_user_auth.sql
mariadb -u oops_reader -p'strong_password' oops_reader < migrations/002_init_books.sql
mariadb -u oops_reader -p'strong_password' oops_reader < migrations/003_init_user_reading_data.sql
mariadb -u oops_reader -p'strong_password' oops_reader < migrations/004_init_sync_and_normalization.sql

# 验证表创建
mariadb -u oops_reader -p'strong_password' oops_reader -e "SHOW TABLES;"
```

#### 4.3 验证数据库

```sql
-- 检查表数量
SELECT COUNT(*) AS table_count FROM information_schema.tables
WHERE table_schema = 'oops_reader';
-- 预期输出: 18

-- 检查关键表
SELECT table_name, table_rows
FROM information_schema.tables
WHERE table_schema = 'oops_reader'
ORDER BY table_name;
```

---

### 5. 服务配置

#### 5.1 修改配置文件

创建 `/opt/oops-reader-backend/config.yaml`:

```yaml
server:
  port: 8080
  mode: release
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 30s

database:
  host: localhost
  port: 3306
  user: oops_reader
  password: "your_strong_password_here"
  database: oops_reader
  max_open_conns: 100
  max_idle_conns: 10
  conn_max_lifetime: 1h
  conn_max_idle_time: 10m

jwt:
  secret: "your-super-secret-key-change-in-production-32chars"
  access_token_expiry: 1h
  refresh_token_expiry: 168h

log:
  level: info
  format: json
  output_path: /var/log/oops-reader-backend/app.log
```

#### 5.2 创建日志目录

```bash
sudo mkdir -p /var/log/oops-reader-backend
sudo chown -R oops-reader:oops-reader /var/log/oops-reader-backend
```

#### 5.3 配置 Systemd 服务

创建 `/etc/systemd/system/oops-reader.service`:

```ini
[Unit]
Description=Oops Reader Backend API Server
After=network.target mariadb.service

[Service]
Type=simple
User=oops-reader
Group=oops-reader
WorkingDirectory=/opt/oops-reader-backend
Environment="PATH=/opt/oops-reader-backend/.venv/bin:/usr/local/bin:/usr/bin:/bin"
Environment="GOPROXY=https://goproxy.cn,direct"
ExecStart=/opt/oops-reader-backend/bin/simple
Restart=always
RestartSec=5
StandardOutput=append:/var/log/oops-reader-backend/service.log
StandardError=append:/var/log/oops-reader-backend/error.log

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/oops-reader-backend

[Install]
WantedBy=multi-user.target
```

---

### 6. 启动服务

#### 6.1 构建项目

```bash
cd /opt/oops-reader-backend

# 使用 oops-reader 用户构建
sudo -u oops-reader go build -o bin/simple ./cmd/simple

# 验证构建
ls -lh bin/simple
```

#### 6.2 启动服务

```bash
# 重新加载 systemd 配置
sudo systemctl daemon-reload

# 启用服务（开机自启）
sudo systemctl enable oops-reader

# 启动服务
sudo systemctl start oops-reader

# 检查状态
sudo systemctl status oops-reader
```

#### 6.3 验证服务

```bash
# 检查服务状态
sudo systemctl status oops-reader

# 查看日志
sudo journalctl -u oops-reader -f

# 查看应用日志
sudo tail -f /var/log/oops-reader-backend/service.log

# 测试健康检查接口
curl http://localhost:8080/health
```

#### 6.4 测试 API

```bash
# 健康检查
curl http://localhost:8080/health

# 书籍解析
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "三体"}'

# 书籍封面
curl "http://localhost:8080/v1/utils/book-cover?book_name=三体"
```

---

## Docker 部署

### 1. 安装 Docker

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install -y docker.io docker-compose

sudo systemctl start docker
sudo systemctl enable docker

sudo usermod -aG docker oops-reader
```

**CentOS/RHEL:**
```bash
sudo yum install -y docker docker-compose

sudo systemctl start docker
sudo systemctl enable docker

sudo usermod -aG docker oops-reader
```

### 2. 创建 Dockerfile

创建 `/opt/oops-reader-backend/Dockerfile`:

```dockerfile
# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /build

# 设置代理（可选）
ENV GOPROXY=https://goproxy.cn,direct

# 复制依赖文件
COPY go.mod go.sum* ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/simple ./cmd/simple

# 运行阶段
FROM alpine:3.19

# 安装 Python 和依赖
RUN apk add --no-cache python3 py3-pip ca-certificates tzdata

# 安装 Python 依赖
RUN pip3 install --no-cache-dir requests

# 设置时区
ENV TZ=Asia/Shanghai

# 创建工作目录
WORKDIR /app

# 从构建阶段复制文件
COPY --from=builder /build/bin/simple /app/bin/simple
COPY --from=builder /build/utils /app/utils
COPY --from=builder /build/config.yaml /app/config.yaml

# 创建非 root 用户
RUN addgroup -g 1000 oops-reader && \
    adduser -D -u 1000 -G oops-reader oops-reader

# 创建日志目录
RUN mkdir -p /var/log/oops-reader-backend && \
    chown -R oops-reader:oops-reader /var/log/oops-reader-backend

# 切换到非 root 用户
USER oops-reader

# 暴露端口
EXPOSE 8080

# 启动应用
CMD ["/app/bin/simple"]
```

### 3. 创建 docker-compose.yml

创建 `/opt/oops-reader-backend/docker-compose.yml`:

```yaml
version: '3.8'

services:
  mariadb:
    image: mariadb:10.11
    container_name: oops-reader-mariadb
    environment:
      MARIADB_ROOT_PASSWORD: root_password
      MARIADB_DATABASE: oops_reader
      MARIADB_USER: oops_reader
      MARIADB_PASSWORD: oops_reader_password
    ports:
      - "3306:3306"
    volumes:
      - mariadb_data:/var/lib/mysql
      - ./migrations:/docker-entrypoint-initdb.d:ro
    command:
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_unicode_ci
    networks:
      - oops-network

  app:
    build: .
    container_name: oops-reader-backend
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - app_logs:/var/log/oops-reader-backend
    depends_on:
      - mariadb
    restart: unless-stopped
    networks:
      - oops-network

volumes:
  mariadb_data:
  app_logs:

networks:
  oops-network:
    driver: bridge
```

### 4. 构建和运行

```bash
cd /opt/oops-reader-backend

# 构建镜像
sudo docker-compose build

# 启动服务
sudo docker-compose up -d

# 查看日志
sudo docker-compose logs -f app

# 查看状态
sudo docker-compose ps
```

### 5. Docker 管理命令

```bash
# 停止服务
sudo docker-compose stop

# 重启服务
sudo docker-compose restart

# 查看日志
sudo docker-compose logs -f app

# 进入容器
sudo docker exec -it oops-reader-backend sh

# 删除容器和数据卷
sudo docker-compose down -v

# 更新并启动
sudo docker-compose pull
sudo docker-compose up -d
```

---

## Nginx 反向代理

### 1. 安装 Nginx

**Ubuntu/Debian:**
```bash
sudo apt install -y nginx
```

**CentOS/RHEL:**
```bash
sudo yum install -y nginx
```

### 2. 配置 Nginx

创建 `/etc/nginx/sites-available/oops-reader`:

```nginx
upstream oops_reader_backend {
    server localhost:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name your-domain.com;  # 替换为你的域名

    # 日志
    access_log /var/log/nginx/oops-reader-access.log;
    error_log /var/log/nginx/oops-reader-error.log;

    # 客户端最大请求体大小
    client_max_body_size 10M;

    # Gzip 压缩
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript
               application/json application/javascript application/xml+rss;
    gzip_disable "msie6";

    # 代理设置
    location / {
        proxy_pass http://oops_reader_backend;
        proxy_http_version 1.1;

        # 请求头
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # 超时设置
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 60s;

        # 缓冲设置
        proxy_buffering on;
        proxy_buffer_size 4k;
        proxy_buffers 8 4k;
        proxy_busy_buffers_size 8k;
    }

    # 健康检查端点不缓存
    location /health {
        proxy_pass http://oops_reader_backend/health;
        proxy_http_version 1.1;
        proxy_cache_bypass $http_upgrade;
        add_header Cache-Control "no-cache, no-store, must-revalidate";
    }
}
```

### 3. 启用配置

```bash
# 创建软链接
sudo ln -s /etc/nginx/sites-available/oops-reader /etc/nginx/sites-enabled/

# 测试配置
sudo nginx -t

# 重新加载 Nginx
sudo systemctl reload nginx
```

### 4. HTTPS 配置（可选）

使用 Let's Encrypt 免费 SSL 证书：

```bash
# 安装 certbot
sudo apt install -y certbot python3-certbot-nginx

# 自动配置 SSL
sudo certbot --nginx -d your-domain.com

# 自动续期
sudo certbot renew --dry-run
```

---

## 监控和维护

### 1. 日志管理

#### 查看应用日志

```bash
# 实时查看
sudo journalctl -u oops-reader -f

# 查看最近100行
sudo journalctl -u oops-reader -n 100

# 查看今天的日志
sudo journalctl -u oops-reader --since today

# 查看错误日志
sudo journalctl -u oops-reader -p err
```

#### 日志轮转

创建 `/etc/logrotate.d/oops-reader`:

```
/var/log/oops-reader-backend/*.log {
    daily
    missingok
    rotate 14
    compress
    delaycompress
    notifempty
    create 644 oops-reader oops-reader
    sharedscripts
    postrotate
        systemctl reload oops-reader > /dev/null 2>&1 || true
    endscript
}
```

### 2. 性能监控

#### 系统资源监控

```bash
# CPU 和内存使用
top
htop

# 磁盘使用
df -h
du -sh /opt/oops-reader-backend

# 网络连接
netstat -tlnp | grep 8080
ss -tlnp | grep 8080
```

#### 服务健康检查

创建 `/usr/local/bin/check-oops-reader.sh`:

```bash
#!/bin/bash

# 健康检查
HEALTH_CHECK=$(curl -s http://localhost:8080/health)
if [ $? -eq 0 ]; then
    echo "$(date): Service is healthy" >> /var/log/oops-reader-health.log
else
    echo "$(date): Service is DOWN" >> /var/log/oops-reader-health.log
    # 可选：发送通知或自动重启
    # systemctl restart oops-reader
fi

chmod +x /usr/local/bin/check-oops-reader.sh

# 添加到 crontab
*/5 * * * * /usr/local/bin/check-oops-reader.sh
```

#### API 响应时间监控

```bash
#!/bin/bash

# 测试接口响应时间
echo "Health Check: $(curl -o /dev/null -s -w '%{time_total}\n' http://localhost:8080/health)"
echo "Parse Book Info: $(curl -o /dev/null -s -w '%{time_total}\n' -X POST http://localhost:8080/v1/utils/parse-book-info -H 'Content-Type: application/json' -d '{"input":"test"}')"
echo "Get Book Cover: $(curl -o /dev/null -s -w '%{time_total}\n' 'http://localhost:8080/v1/utils/book-cover?book_name=test')"
```

### 3. 备份

#### 数据库备份

创建 `/usr/local/bin/backup-mariadb.sh`:

```bash
#!/bin/bash

BACKUP_DIR="/var/backups/mariadb"
DATE=$(date +%Y%m%d_%H%M%S)
MARIADB_USER="oops_reader"
MARIADB_PASS="your_password"
MARIADB_DB="oops_reader"

mkdir -p $BACKUP_DIR

# 备份数据库
mariadb-dump -u$MARIADB_USER -p$MARIADB_PASS $MARIADB_DB | gzip > $BACKUP_DIR/oops_reader_$DATE.sql.gz

# 删除30天前的备份
find $BACKUP_DIR -name "oops_reader_*.sql.gz" -mtime +30 -delete

echo "Backup completed: oops_reader_$DATE.sql.gz"

chmod +x /usr/local/bin/backup-mariadb.sh

# 添加到 crontab（每天凌晨2点）
0 2 * * * /usr/local/bin/backup-mariadb.sh
```

#### 恢复数据库

```bash
# 列出备份
ls -lh /var/backups/mariadb/

# 恢复数据
gunzip < /var/backups/mariadb/oops_reader_20251616_020000.sql.gz | mariadb -u oops_reader -p oops_reader
```

---

## 故障排查

### 1. 服务无法启动

#### 检查服务状态

```bash
sudo systemctl status oops-reader
```

#### 查看详细日志

```bash
# Systemd 日志
sudo journalctl -u oops-reader -n 50 --no-pager

# 应用日志
sudo tail -50 /var/log/oops-reader-backend/service.log
sudo tail -50 /var/log/oops-reader-backend/error.log
```

#### 常见问题

**问题 1: 端口被占用**
```bash
# 查看端口占用
sudo lsof -i:8080
# 或
sudo netstat -tlnp | grep 8080

# 停止占用进程
sudo kill <PID>
```

**问题 2: 权限问题**
```bash
# 检查文件权限
ls -la /opt/oops-reader-backend

# 修复权限
sudo chown -R oops-reader:oops-reader /opt/oops-reader-backend
sudo chmod 755 /opt/oops-reader-backend/bin/simple
```

**问题 3: 数据库连接失败**
```bash
# 测试数据库连接
mariadb -u oops_reader -p oops_reader

# 检查 MariaDB 状态
sudo systemctl status mariadb

# 查看 MariaDB 日志
sudo tail -50 /var/log/mariadb/mariadb.log
```

### 2. API 响应慢

#### 检查资源使用

```bash
# CPU 和内存
top -p $(pgrep bin/simple)

# 网络延迟
ping -c 5 www.baidu.com
ping -c 5 books.google.com

# DNS 解析
nslookup books.google.com
```

#### 优化建议

1. **使用 CDN**: 将封面图片缓存到本地或 CDN
2. **增加缓存**: 添加 Redis 缓存层
3. **优化数据库**: 添加索引，优化查询
4. **升级硬件**: 更快的 CPU 和更多内存

### 3. Python 脚本错误

#### 测试 Python 环境

```bash
cd /opt/oops-reader-backend

# 激活虚拟环境
source .venv/bin/activate

# 测试 Python 版本
python --version

# 测试依赖
python -c "import requests; print(requests.__version__)"

# 手动运行脚本
python utils/book_parser.py "测试"
python utils/bookCover.py "测试"

# 退出虚拟环境
deactivate
```

#### 安装缺失的依赖

```bash
cd /opt/oops-reader-backend

# 激活虚拟环境
source .venv/bin/activate

# 安装 requests
pip install requests

# 或使用系统包管理器（可选）
sudo apt install -y python3-requests
```

### 4. 内存或磁盘空间不足

#### 清理日志

```bash
# 清理旧日志
sudo journalctl --vacuum-time=7d

# 清理系统日志
sudo find /var/log -name "*.log" -mtime +30 -delete
```

#### 清理应用日志

```bash
# 查看日志大小
du -sh /var/log/oops-reader-backend

# 清理旧日志（保留最近7天）
find /var/log/oops-reader-backend -name "*.log" -mtime +7 -delete
```

#### 清理磁盘空间

```bash
# 查看磁盘使用
df -h

# 清理 APT 缓存
sudo apt clean
sudo apt autoremove

# 清理 Docker（如果使用）
sudo docker system prune -a
```

---

## 安全加固

### 1. 防火墙配置

```bash
# 安装 UFW（Ubuntu）
sudo apt install -y ufw

# 默认策略
sudo ufw default deny incoming
sudo ufw default allow outgoing

# 允许 SSH
sudo ufw allow 22/tcp

# 允许 HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# 启用防火墙
sudo ufw enable

# 查看状态
sudo ufw status
```

### 2. 限制访问

```nginx
# Nginx 配置 - 限制 IP 访问
server {
    listen 80;

    # 仅允许特定 IP 访问
    allow 192.168.1.0/24;
    allow 10.0.0.0/8;
    deny all;

    # 其他配置...
}
```

### 3. SSL/TLS 加密

```bash
# 使用 Let's Encrypt
sudo certbot --nginx -d your-domain.com

# 配置自动续期
sudo certbot renew --dry-run

# 添加 cron 任务
0 0 * * * certbot renew --quiet --post-hook "systemctl reload nginx"
```

---

## 升级和更新

### 1. 更新代码

```bash
cd /opt/oops-reader-backend

# 激活虚拟环境
source .venv/bin/activate

# 拉取最新代码
sudo -u oops-reader git pull origin main

# 更新 Python 依赖
pip install -r requirements.txt

# 退出虚拟环境
deactivate

# 重新构建
sudo -u oops-reader go build -o bin/simple ./cmd/simple

# 重启服务
sudo systemctl restart oops-reader
```

### 2. 数据库迁移

```bash
# 查看当前迁移版本
mariadb -u oops_reader -p'strong_password' oops_reader -e "SELECT MAX(id) FROM migrations;"

# 应用新迁移
mariadb -u oops_reader -p'strong_password' oops_reader < migrations/005_new_feature.sql

# 验证迁移
SHOW TABLES;
```

---

## 附录

### A. 环境变量配置

创建 `/opt/oops-reader-backend/.env`:

```bash
# 服务配置
PORT=8080
MODE=release

# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USER=oops_reader
DB_PASS=your_password
DB_NAME=oops_reader

# JWT 配置
JWT_SECRET=your-super-secret-key-change-in-production

# 日志配置
LOG_LEVEL=info
LOG_FORMAT=json
LOG_PATH=/var/log/oops-reader-backend/app.log
```

### B. 监控脚本

创建 `/usr/local/bin/monitor-oops-reader.sh`:

```bash
#!/bin/bash

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# 检查服务状态
if systemctl is-active --quiet oops-reader; then
    echo -e "${GREEN}✓${NC} Service is running"
else
    echo -e "${RED}✗${NC} Service is NOT running"
    exit 1
fi

# 健康检查
HEALTH=$(curl -s http://localhost:8080/health)
if echo "$HEALTH" | grep -q "ok"; then
    echo -e "${GREEN}✓${NC} Health check passed"
else
    echo -e "${RED}✗${NC} Health check failed"
    exit 1
fi

# 内存使用
MEM=$(ps aux | grep 'bin/simple' | grep -v grep | awk '{sum+=$4} END {print sum}')
echo "Memory usage: ${MEM}%"

# 检查磁盘空间
DISK=$(df /opt | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $DISK -gt 80 ]; then
    echo -e "${RED}✗${NC} Disk usage: ${DISK}% (high)"
else
    echo -e "${GREEN}✓${NC} Disk usage: ${DISK}%"
fi

echo "All checks completed!"

chmod +x /usr/local/bin/monitor-oops-reader.sh
```

### C. 快速重启脚本

创建 `/usr/local/bin/restart-oops-reader.sh`:

```bash
#!/bin/bash

echo "Restarting Oops Reader Backend..."

# 停止服务
sudo systemctl stop oops-reader

# 等待完全停止
sleep 2

# 启动服务
sudo systemctl start oops-reader

# 等待启动
sleep 3

# 检查状态
if systemctl is-active --quiet oops-reader; then
    echo "✓ Service restarted successfully"

    # 测试健康检查
    HEALTH=$(curl -s http://localhost:8080/health)
    echo "Health check: $HEALTH"
else
    echo "✗ Failed to restart service"
    sudo journalctl -u oops-reader -n 50
    exit 1
fi

chmod +x /usr/local/bin/restart-oops-reader.sh
```

---

**文档更新**: 2025-03-16
**适用版本**: v1.0.0
**维护者**: Oops Reader Team

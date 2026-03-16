# Git Ignore 规则说明

本文档说明 Oops Reader Backend 项目的 `.gitignore` 配置及其使用。

---

## 概述

`.gitignore` 文件指定了 Git 应该忽略的文件和目录，这些文件不会被版本控制系统跟踪。

**文件位置**: `.gitignore`（项目根目录）

---

## 被忽略的文件类型

### 1. Go 相关

| 类型 | 模式 | 说明 |
|------|------|------|
| 可执行文件 | `*.exe`, `*.dll`, `*.so`, `*.dylib` | 编译产生的二进制文件 |
| 测试文件 | `*.test` | Go 测试编译产物 |
| 覆盖率报告 | `*.out` | Go coverage 输出 |
| 工作区文件 | `go.work` | Go workspace 配置 |
| 依赖目录 | `vendor/` | 第三方依赖 |
| 构建目录 | `/bin/`, `/dist/`, `.GOPATH/` | 构建输出目录 |
| 依赖摘要 | `go.sum` | Go 模块校验和 |

### 2. 配置和敏感信息

| 类型 | 模式 | 说明 |
|------|------|------|
| 环境变量 | `.env`, `.env.local`, `.env.*.local` | 环境变量文件 |
| 应用配置 | `/config.yaml` | 应用配置（包含密码） |
| 密钥 | `/secrets.yaml`, `*.pem`, `*.key`, `*.crt` | 证书和密钥文件 |
| 凭证 | `credentials.json` | API 凭证文件 |

**重要**: `config.yaml` 被忽略，但 `config.yaml.example` 会被提交。

### 3. Python 相关

| 类型 | 模式 | 说明 |
|------|------|------|
| 字节码 | `__pycache__/`, `*.pyc`, `*.pyo` | Python 编译缓存 |
| 虚拟环境 | `venv/`, `ENV/`, `env/`, `.venv/` | Python 虚拟环境 |
| 测试缓存 | `.pytest_cache/` | Pytest 缓存目录 |
| 覆盖率 | `.coverage`, `htmlcov/`, `coverage.txt` | 测试覆盖率报告 |
| 构建产物 | `build/`, `dist/`, `*.egg-info/` | Python 包构建产物 |

### 4. IDE 和编辑器

| 编辑器 | 模式 |
|--------|------|
| VSCode | `.vscode/` |
| IntelliJ IDEA | `.idea/` |
| Vim | `*.swp`, `*.swo` |
| 临时文件 | `*~` |

### 5. 日志文件

| 类型 | 模式 | 说明 |
|------|------|------|
| 应用日志 | `*.log`, `logs/`, `/log/` | 应用运行日志 |
| 日志轮转 | `*.log.*` | 日志备份文件 |

### 6. 数据库文件

| 类型 | 模式 | 说明 |
|------|------|------|
| SQLite | `*.sqlite`, `*.sqlite3` | SQLite 数据库文件 |
| 其他数据库 | `*.db` | 通用数据库文件 |

### 7. 临时文件

| 类型 | 模式 | 说明 |
|------|------|------|
| 临时目录 | `tmp/`, `temp/` | 临时文件目录 |
| 临时文件 | `*.tmp`, `*.temp` | 临时文件 |

### 8. 操作系统文件

| 系统 | 模式 |
|------|------|
| macOS | `.DS_Store`, `.Trashes` |
| Windows | `ehthumbs.db`, `Thumbs.db` |

### 9. 应用特定文件

| 类型 | 模式 | 说明 |
|------|------|------|
| 缓存 | `.cache/`, `cache/` | 应用缓存目录 |
| 备份 | `*.bak`, `*.backup` | 备份文件 |
| 上传 | `uploads/`, `generated/` | 用户上传和生成的内容 |

### 10. 项目特定数据

| 类型 | 模式 | 说明 |
|------|------|------|
| 书籍数据 | `utils/book_data/*`, `utils/book_ranks/*` | 抓取的书籍数据 |
| 数据目录 | `/data/` | 应用数据目录 |

**保留**: `.gitkeep` 文件用于保持空目录结构。

---

## 当前被忽略的文件

以下文件在实际项目中被忽略：

```
bin/                    # 编译输出目录
config.yaml            # 配置文件
data/                  # 数据目录
go.sum                 # 依赖摘要
utils/__pycache__/     # Python 缓存
utils/book_data/       # 书籍数据
utils/book_ranks/      # 书籍排行
```

**验证命令：**
```bash
git status --ignored --porcelain | grep "^!!"
```

---

## 使用指南

### 1. 首次部署配置

从示例配置创建实际配置：

```bash
cp config.yaml.example config.yaml
nano config.yaml
```

**重要**: `config.yaml.example` 会被提交到 Git，可以安全分享。

### 2. 添加新忽略规则

编辑 `.gitignore` 文件：

```bash
nano .gitignore
```

**示例**：
```bash
# 添加新的忽略规则
*.secret
/private/
```

### 3. 移除已跟踪的文件

如果文件已经被 Git 跟踪，需要先移除跟踪：

```bash
# 从 Git 跟踪中移除，但保留本地文件
git rm --cached config.yaml

# 提交变更
git commit -m "Remove config.yaml from version control"

# 确认 config.yaml 在 .gitignore 中
echo "/config.yaml" >> .gitignore
git add .gitignore
git commit -m "Add config.yaml to .gitignore"
```

### 4. 验证忽略规则

检查特定文件是否被忽略：

```bash
# 检查单个文件
git check-ignore -v config.yaml

# 检查所有被忽略的文件
git status --ignored
```

### 5. 调试忽略规则

查看为什么文件被忽略或未被忽略：

```bash
# 查看文件的 .gitignore 规则来源
git check-ignore -v --config config.yaml

# 查看所有忽略规则的来源
git check-ignore -v --stdin < .gitignore
```

---

## 模式语法

### 基本模式

- `# 开头`: 注释
- `*`: 匹配任意字符
- `?`: 匹配单个字符
- `[]`: 匹配范围内的字符
- `!`: 取反（不忽略）

### 示例

| 模式 | 说明 | 匹配示例 |
|------|------|---------|
| `*.log` | 忽略所有 .log 文件 | `app.log`, `error.log` |
| `/config.yaml` | 忽略根目录的 config.yaml | `config.yaml`（根目录） |
| `config.yaml` | 忽略任意位置的 config.yaml | `config.yaml`, `dir/config.yaml` |
| `!/config.yaml.example` | 不忽略 config.yaml.example | `config.yaml.example` |
| `/bin/` | 忽略 bin 目录 | `bin/`, `bin/api` |
| `*.py[cod]` | 忽略 .pyc, .pyo, .pyd | `main.pyc`, `test.pyo` |

### 目录规则

- `dir/`: 忽略 dir 目录及其内容
- `dir`: 忽略 dir 文件或目录
- `/dir`: 只忽略根目录下的 dir

### 前导斜杠

- 带 `/`: 从项目根目录匹配
- 不带 `/`: 从任何目录匹配

---

## 最佳实践

### 1. 保持 .gitignore 简洁

```bash
# ✅ 好的规则
*.log
.env

# ❌ 不必要
secret-data-file.txt  # 不应该出现在代码库中
```

### 2. 使用注释解释复杂规则

```bash
# 配置文件（包含敏感信息）
/config.yaml
/secrets/

# 保留示例配置
!/config.yaml.example
```

### 3. 按类型组织规则

建议将同类规则放在一起：

```bash
# ===编译产物===
*.exe
*.dll
/bin/

# ===敏感信息===
.env
config.yaml

# ===临时文件===
*.tmp
*.swp
```

### 4. 使用全局 .gitignore（可选）

可以为所有项目设置全局忽略规则：

```bash
# 创建全局 .gitignore
git config --global core.excludesfile ~/.gitignore

# 编辑全局 .gitignore
nano ~/.gitignore

# 添加常用规则
.DS_Store
Thumbs.db
*.swp
*~
```

### 5. 定期审查 .gitignore

定期检查 `.gitignore` 是否需要更新：

- 新增的文件类型
- 新增的工具或框架
- 过时的规则

---

## 依赖管理

### Go 依赖

**为什么不提交 vendor/：**
- 增加仓库大小
- Go 1.11+ 的模块系统已足够
- `go.mod` 和 `go.sum` 已足够

**需要重新获取依赖时：**
```bash
go mod download
```

### Python 依赖

**为什么不提交 venv/：**
- 虚拟环境不能跨平台
- 应使用 `requirements.txt` 管理依赖

**创建虚拟环境：**
```bash
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

---

## 团队协作

### 1. 统一 .gitignore

确保团队成员使用一致的 `.gitignore`：

```bash
# 更新 .gitignore
echo "*.secret" >> .gitignore
git add .gitignore
git commit -m "Update .gitignore"
git push
```

### 2. 新成员入职

提供配置模板：

```bash
# 1. 克隆项目
git clone https://github.com/oops-reader/oops-reader-backend.git
cd oops-reader-backend

# 2. 创建配置文件
cp config.yaml.example config.yaml

# 3. 编辑配置
nano config.yaml

# 4. 安装依赖
go mod download
pip3 install requests

# 5. 构建并运行
go build -o bin/simple ./cmd/simple
./bin/simple
```

### 3. 敏感信息管理

**不可提交的内容：**
- 数据库密码
- API 密钥
- JWT 签名密钥
- 云服务凭证
- SSL/TLS 证书私钥

**安全做法：**
1. 环境变量
2. 密钥管理系统
3. CI/CD Secrets
4. 配置管理工具（Vault）

---

## 故障排查

### 1. 文件仍然被跟踪

**症状**: 已添加到 .gitignore，但仍被跟踪

**原因**: 文件之前已被提交到 Git

**解决**:
```bash
# 停止跟踪但保留文件
git rm --cached filename

# 停止跟踪目录
git rm -r --cached directory/

# 提交
git commit -m "Remove file from tracking"
```

### 2. 希望提交被忽略的文件

**症状**: 文件被 .gitignore，但需要提交

**解决**:
```bash
# 强制添加（不推荐）
git add -f filename

# 更好的做法：
# 1. 将文件重命名为不被忽略的名字
# 2. 更新 .gitignore 规则
# 3. 提交正确的版本
```

### 3. 忽略已被修改的文件

**症状**: 文件已被修改，但不想提交更改

**解决**:
```bash
# 假设修改但不想提交
git update-index --assume-unchanged config.yaml

# 恢复跟踪
git update-index --no-assume-unchanged config.yaml
```

### 4. 检查文件状态

```bash
# 显示所有跟踪的文件
git ls-files

# 显示被忽略的文件
git status --ignored --short

# 检查文件是否被忽略
git check-ignore -v filename
```

---

## 相关文档

- [配置文件说明](./config-guide.md)
- [Git 官方文档](https://git-scm.com/docs/gitignore)
- [GitHub .gitignore 模板](https://github.com/github/gitignore)

---

**更新日期**: 2025-03-16
**版本**: 1.0.0

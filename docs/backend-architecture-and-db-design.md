# Gravity Reader Backend 架构设计与数据库设计

## 1. 目标与范围

本项目作为 `~/Workspace/Code/gravity-reader/` 的服务端，负责以下核心能力：

1. 用户账号管理
2. 用户阅读数据同步
3. 书籍信息检索与聚合
4. 用户输入书名的规范化处理

首期覆盖的数据范围包括但不限于：

- 书架书籍
- 阅读进度
- 阅读时长
- 偏好设置
- 读书笔记
- 用户基础信息

技术约束：

- 语言：Golang
- 数据库：MySQL

## 2. 设计原则

1. 先单体、后拆分：首期建议采用模块化单体，降低复杂度，保留未来拆分空间。
2. 数据同步优先可追踪：所有客户端同步行为都要具备版本号、更新时间和幂等能力。
3. 书籍元数据与用户阅读数据解耦：书籍主数据是公共域，用户书架/进度/笔记是私有域。
4. 书名规范化独立建模：保留原始输入、清洗结果、候选匹配、最终标准书籍映射。
5. 外部检索可替换：通过 Provider 抽象对接互联网书籍信息来源，避免被单一来源绑定。

## 3. 推荐总体架构

首期建议采用“模块化单体 + 清晰领域边界”的结构。

### 3.1 逻辑分层

1. API 层
   - 提供 HTTP/JSON 接口
   - 鉴权、参数校验、限流、响应封装

2. Application 层
   - 编排业务流程
   - 控制事务边界
   - 聚合同步、检索、规范化等用例

3. Domain 层
   - 核心领域模型
   - 领域服务
   - 领域规则和状态转换

4. Infrastructure 层
   - MySQL 持久化
   - Redis 缓存（建议）
   - 外部书籍检索 Provider
   - 异步任务队列
   - 日志、监控、配置

### 3.2 核心模块

建议按业务域划分包，而不是按 controller/service/repository 平铺。

- `user`
  - 注册、登录、注销
  - token/session 管理
  - 用户资料维护

- `sync`
  - 客户端增量同步
  - 服务端冲突处理
  - 操作日志与版本推进

- `books`
  - 标准书籍主数据
  - 作者、出版社、封面、ISBN 等元信息
  - 外部来源聚合

- `bookshelf`
  - 用户书架
  - 加书、删书、状态变化
  - 在读/想读/读完/归档

- `reading`
  - 阅读进度
  - 阅读时长
  - 最近阅读时间
  - 阅读统计

- `notes`
  - 读书笔记
  - 划线、摘录、感想

- `preferences`
  - 阅读偏好设置
  - 字体、主题、翻页、同步配置等

- `normalization`
  - 用户输入书名清洗
  - 别名匹配
  - 候选召回
  - 标准书籍映射

- `search`
  - 通过书名检索互联网书籍信息
  - 聚合多个来源结果
  - 排序和去重

## 4. 部署架构建议

首期建议：

- `api-server`
  - Go HTTP 服务
  - 对外提供 RESTful API

- `mysql`
  - 核心业务数据库

- `redis`（强烈建议）
  - 登录态缓存
  - 热门书籍缓存
  - 书名规范化缓存
  - 限流计数

- `worker`
  - 异步处理书籍抓取、封面下载、规范化补全、统计汇总

### 4.1 首期为什么不直接拆微服务

因为当前业务核心是“以用户为中心的阅读同步系统”，模块间强关联明显：

- 书架变化会影响同步
- 同步会触发规范化
- 规范化会关联书籍主数据
- 阅读进度和笔记都归属用户书架

首期拆微服务会显著增加：

- 分布式事务复杂度
- 调试成本
- 接口联调成本
- 运维成本

因此建议：

- 代码按领域模块化
- 部署保持单体
- 通过接口和仓储抽象为未来拆分做准备

## 5. Go 项目结构建议

```text
oops-reader-backend/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── worker/
│       └── main.go
├── internal/
│   ├── user/
│   ├── sync/
│   ├── books/
│   ├── bookshelf/
│   ├── reading/
│   ├── notes/
│   ├── preferences/
│   ├── normalization/
│   ├── search/
│   ├── platform/
│   │   ├── config/
│   │   ├── db/
│   │   ├── cache/
│   │   ├── log/
│   │   ├── auth/
│   │   └── queue/
│   └── transport/
│       └── http/
├── migrations/
├── docs/
└── go.mod
```

## 6. 关键业务流程

### 6.1 用户账号管理

流程：

1. 用户注册或使用第三方登录
2. 服务端创建用户记录
3. 生成 access token / refresh token
4. 初始化默认偏好设置

建议支持：

- 邮箱注册登录
- 手机号注册登录（可后续扩展）
- Apple / Google 第三方登录（如果客户端面向移动端，建议预留）

### 6.2 阅读数据同步

建议采用“客户端操作日志 + 服务端版本号”的增量同步模型。

客户端上传：

- 本地变更操作列表
- 客户端最后已知同步版本
- 设备信息

服务端处理：

1. 校验用户身份
2. 按幂等键去重
3. 写入用户域表
4. 记录同步事件
5. 返回服务端自上次版本后的增量数据

优点：

- 适合多端同步
- 易于冲突处理
- 易于审计与恢复

### 6.3 书名检索与规范化

分为两个阶段：

1. 在线检索
   - 用户输入书名
   - 查询本地标准书库
   - 命中不足时调用外部 Provider
   - 聚合并返回候选结果

2. 离线规范化
   - 保存用户原始输入
   - 清洗标点、空格、副标题、括号内容
   - 建立 alias 和标准书籍映射
   - 对高频未命中输入做人工/规则回填

## 7. 数据库设计

数据库建议分为四类表：

1. 用户与认证
2. 书籍主数据
3. 用户阅读数据
4. 同步与规范化

以下字段类型以 MySQL 8.0 为基准。

---

## 8. 用户与认证表

### 8.1 `users`

用户主表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 用户 ID |
| email | varchar(128) unique null | 邮箱 |
| phone | varchar(32) unique null | 手机号 |
| password_hash | varchar(255) null | 密码哈希 |
| nickname | varchar(64) not null | 昵称 |
| avatar_url | varchar(512) null | 头像 |
| status | tinyint not null default 1 | 1=正常 2=冻结 3=注销 |
| last_login_at | datetime null | 最近登录时间 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `uk_email (email)`
- `uk_phone (phone)`
- `idx_status_created_at (status, created_at)`

### 8.2 `user_auth_providers`

第三方登录绑定表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| provider | varchar(32) not null | google/apple/wechat 等 |
| provider_user_id | varchar(128) not null | 第三方用户 ID |
| provider_union_id | varchar(128) null | 可选统一 ID |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `uk_provider_uid (provider, provider_user_id)`
- `idx_user_id (user_id)`

### 8.3 `user_sessions`

登录会话表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| device_id | varchar(128) not null | 设备 ID |
| device_name | varchar(128) null | 设备名称 |
| platform | varchar(32) not null | ios/android/web/mac 等 |
| refresh_token_hash | varchar(255) not null | refresh token 哈希 |
| expires_at | datetime not null | 过期时间 |
| last_active_at | datetime not null | 最近活跃时间 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `idx_user_id (user_id)`
- `idx_device_id (device_id)`
- `idx_expires_at (expires_at)`

---

## 9. 书籍主数据表

### 9.1 `books`

标准书籍主表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 书籍 ID |
| canonical_title | varchar(255) not null | 标准书名 |
| subtitle | varchar(255) null | 副标题 |
| original_title | varchar(255) null | 原书名 |
| description | text null | 简介 |
| publisher | varchar(128) null | 出版社 |
| publish_date | date null | 出版日期 |
| isbn10 | varchar(16) null | ISBN-10 |
| isbn13 | varchar(20) null | ISBN-13 |
| page_count | int unsigned null | 页数 |
| language | varchar(32) null | 语言 |
| cover_url | varchar(512) null | 封面地址 |
| source_confidence | decimal(5,2) not null default 0 | 元数据可信度 |
| status | tinyint not null default 1 | 1=有效 2=待核验 3=下线 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `uk_isbn13 (isbn13)`
- `uk_isbn10 (isbn10)`
- `idx_title (canonical_title)`
- `idx_publisher_publish_date (publisher, publish_date)`

### 9.2 `authors`

作者表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 作者 ID |
| name | varchar(128) not null | 作者名 |
| name_normalized | varchar(128) not null | 规范化作者名 |
| bio | text null | 简介 |
| avatar_url | varchar(512) null | 头像 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `idx_name_normalized (name_normalized)`

### 9.3 `book_authors`

书籍与作者多对多关联。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| book_id | bigint unsigned not null | 书籍 ID |
| author_id | bigint unsigned not null | 作者 ID |
| role | varchar(32) not null default 'author' | author/translator/editor |
| sort_order | int unsigned not null default 0 | 展示顺序 |
| created_at | datetime not null | 创建时间 |

索引建议：

- `uk_book_author_role (book_id, author_id, role)`
- `idx_author_id (author_id)`

### 9.4 `book_aliases`

书名别名表，用于规范化匹配。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| book_id | bigint unsigned not null | 标准书籍 ID |
| alias_title | varchar(255) not null | 别名 |
| alias_title_normalized | varchar(255) not null | 规范化别名 |
| alias_type | varchar(32) not null | original/translated/common/user_input |
| source | varchar(32) not null | system/provider/manual |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `idx_alias_title_normalized (alias_title_normalized)`
- `idx_book_id (book_id)`

### 9.5 `book_external_sources`

外部书籍来源映射表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| book_id | bigint unsigned not null | 本地书籍 ID |
| source_name | varchar(32) not null | douban/google_books/openlibrary 等 |
| external_book_id | varchar(128) not null | 外部书籍 ID |
| raw_payload | json null | 原始返回数据 |
| sync_status | tinyint not null default 1 | 1=有效 2=待更新 3=失效 |
| last_synced_at | datetime null | 最近同步时间 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `uk_source_external_id (source_name, external_book_id)`
- `idx_book_id (book_id)`

---

## 10. 用户阅读数据表

### 10.1 `user_bookshelves`

用户书架表，是用户和书籍的核心关联表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| book_id | bigint unsigned not null | 书籍 ID |
| shelf_status | varchar(32) not null | want_to_read/reading/finished/archived |
| source_type | varchar(32) not null default 'manual' | manual/import/search |
| rating | tinyint unsigned null | 用户评分 1-10 |
| started_at | datetime null | 开始阅读时间 |
| finished_at | datetime null | 完成阅读时间 |
| last_read_at | datetime null | 最近阅读时间 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |
| deleted_at | datetime null | 软删除时间 |

索引建议：

- `uk_user_book (user_id, book_id)`
- `idx_user_status_last_read (user_id, shelf_status, last_read_at)`
- `idx_book_id (book_id)`

### 10.2 `reading_progress`

阅读进度表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| book_id | bigint unsigned not null | 书籍 ID |
| progress_type | varchar(32) not null | page/chapter/percent/location |
| progress_value | varchar(64) not null | 进度值 |
| progress_percent | decimal(5,2) null | 百分比进度 |
| chapter_title | varchar(255) null | 当前章节 |
| position_cfi | varchar(1024) null | EPUB 定位信息 |
| updated_by_device_id | varchar(128) null | 更新设备 |
| recorded_at | datetime not null | 进度产生时间 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `uk_user_book (user_id, book_id)`
- `idx_user_recorded_at (user_id, recorded_at)`

### 10.3 `reading_sessions`

阅读时长明细表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| book_id | bigint unsigned not null | 书籍 ID |
| device_id | varchar(128) null | 设备 ID |
| started_at | datetime not null | 会话开始时间 |
| ended_at | datetime not null | 会话结束时间 |
| duration_seconds | int unsigned not null | 阅读秒数 |
| created_at | datetime not null | 创建时间 |

索引建议：

- `idx_user_started_at (user_id, started_at)`
- `idx_user_book_started_at (user_id, book_id, started_at)`

### 10.4 `reading_daily_stats`

用户阅读日统计表，用于快速查询。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| stat_date | date not null | 统计日期 |
| total_reading_seconds | int unsigned not null default 0 | 总阅读时长 |
| total_books_read | int unsigned not null default 0 | 当日阅读书籍数 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `uk_user_date (user_id, stat_date)`

### 10.5 `user_notes`

读书笔记表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| book_id | bigint unsigned not null | 书籍 ID |
| note_type | varchar(32) not null | note/highlight/thought |
| quote_text | text null | 摘录内容 |
| note_text | text null | 笔记内容 |
| chapter_title | varchar(255) null | 章节名 |
| position_ref | varchar(1024) null | 位置引用 |
| color | varchar(32) null | 高亮颜色 |
| created_by_device_id | varchar(128) null | 设备 ID |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |
| deleted_at | datetime null | 软删除时间 |

索引建议：

- `idx_user_book_created_at (user_id, book_id, created_at)`
- `idx_user_created_at (user_id, created_at)`

### 10.6 `user_preferences`

用户偏好设置表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| theme | varchar(32) null | light/dark/sepia |
| font_family | varchar(64) null | 字体 |
| font_size | int unsigned null | 字号 |
| line_height | decimal(4,2) null | 行高 |
| page_animation | varchar(32) null | 翻页动画 |
| sync_enabled | tinyint not null default 1 | 是否开启同步 |
| push_enabled | tinyint not null default 1 | 是否开启推送 |
| extra_settings | json null | 扩展配置 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `uk_user_id (user_id)`

---

## 11. 同步与规范化表

### 11.1 `sync_operations`

客户端同步操作日志表，支持幂等和审计。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| device_id | varchar(128) not null | 设备 ID |
| operation_id | varchar(128) not null | 客户端操作唯一 ID |
| entity_type | varchar(32) not null | bookshelf/progress/note/preference |
| entity_id | varchar(128) not null | 实体标识 |
| operation_type | varchar(32) not null | create/update/delete |
| payload | json not null | 操作内容 |
| client_occurred_at | datetime not null | 客户端发生时间 |
| server_version | bigint unsigned not null | 服务端版本号 |
| created_at | datetime not null | 创建时间 |

索引建议：

- `uk_user_device_op (user_id, device_id, operation_id)`
- `idx_user_version (user_id, server_version)`
- `idx_user_entity (user_id, entity_type, entity_id)`

### 11.2 `sync_cursors`

每个用户或设备的同步游标。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned not null | 用户 ID |
| device_id | varchar(128) not null | 设备 ID |
| last_pulled_version | bigint unsigned not null default 0 | 最近拉取版本 |
| last_pushed_at | datetime null | 最近推送时间 |
| last_pulled_at | datetime null | 最近拉取时间 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `uk_user_device (user_id, device_id)`

### 11.3 `book_title_normalization_tasks`

书名规范化任务表。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned null | 来源用户 ID |
| raw_title | varchar(255) not null | 原始书名输入 |
| normalized_title | varchar(255) not null | 清洗后标题 |
| matched_book_id | bigint unsigned null | 匹配到的标准书籍 ID |
| status | tinyint not null default 0 | 0=待处理 1=已匹配 2=待人工确认 3=忽略 |
| confidence_score | decimal(5,2) not null default 0 | 匹配置信度 |
| source | varchar(32) not null | search/import/manual |
| context_payload | json null | 上下文信息 |
| created_at | datetime not null | 创建时间 |
| updated_at | datetime not null | 更新时间 |

索引建议：

- `idx_normalized_title (normalized_title)`
- `idx_status_created_at (status, created_at)`
- `idx_matched_book_id (matched_book_id)`

### 11.4 `book_search_logs`

书籍检索日志表，用于优化召回和排序。

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint unsigned PK | 主键 |
| user_id | bigint unsigned null | 用户 ID |
| query_text | varchar(255) not null | 原始查询词 |
| normalized_query | varchar(255) not null | 规范化查询词 |
| result_count | int unsigned not null default 0 | 结果数 |
| selected_book_id | bigint unsigned null | 最终选择书籍 |
| source | varchar(32) not null | local/mixed/external |
| created_at | datetime not null | 创建时间 |

索引建议：

- `idx_normalized_query_created_at (normalized_query, created_at)`
- `idx_user_created_at (user_id, created_at)`

---

## 12. 表关系摘要

核心关系如下：

- `users` 1:N `user_sessions`
- `users` 1:N `user_bookshelves`
- `users` 1:N `reading_progress`
- `users` 1:N `reading_sessions`
- `users` 1:N `user_notes`
- `users` 1:1 `user_preferences`
- `books` 1:N `book_aliases`
- `books` 1:N `book_external_sources`
- `books` N:M `authors` 通过 `book_authors`
- `users` 与 `books` 通过 `user_bookshelves` 建立关联

## 13. 关键索引与约束建议

1. 用户私有数据表必须带 `user_id` 前缀索引，保证按用户查询性能。
2. 所有同步类表必须有幂等唯一键，例如 `(user_id, device_id, operation_id)`。
3. 高频查询字段要避免全文扫描，例如：
   - 书架状态
   - 最近阅读时间
   - 规范化标题
   - 外部来源 ID
4. 软删除表建议保留 `deleted_at`，避免误删带来的同步冲突。
5. 书籍主数据表与用户数据表分离，避免更新公共元数据时影响用户侧事务。

## 14. API 设计建议

建议首期 REST API，大致如下：

- `POST /v1/auth/register`
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `GET /v1/users/me`
- `PATCH /v1/users/me`

- `GET /v1/books/search`
- `GET /v1/books/{id}`

- `GET /v1/bookshelf`
- `POST /v1/bookshelf`
- `PATCH /v1/bookshelf/{id}`
- `DELETE /v1/bookshelf/{id}`

- `PUT /v1/reading/progress`
- `POST /v1/reading/sessions`
- `GET /v1/reading/stats/daily`

- `GET /v1/notes`
- `POST /v1/notes`
- `PATCH /v1/notes/{id}`
- `DELETE /v1/notes/{id}`

- `GET /v1/preferences`
- `PUT /v1/preferences`

- `POST /v1/sync/push`
- `POST /v1/sync/pull`

## 15. 技术选型建议

Go 侧建议：

- Web 框架：`gin` 或 `fiber`
- ORM / SQL：优先 `sqlc` + `mysql driver`，或者 `gorm`
- 配置：`viper` 或轻量自定义配置
- 日志：`zap`
- 任务队列：`asynq` 或基于 Redis 的轻量任务模型
- 认证：JWT + refresh token

更推荐的实现组合：

- HTTP：`gin`
- DB：`sqlc` + `github.com/go-sql-driver/mysql`
- Cache/Queue：Redis
- Log：`zap`

原因：

- `sqlc` 更适合对 MySQL 表结构和 SQL 性能进行精细控制
- 阅读同步和复杂查询较多，直接控制 SQL 更稳妥
- 后续做统计、同步和规范化任务时更容易优化

## 16. 首期开发优先级

### P0

1. 用户注册/登录/刷新 token
2. 用户信息接口
3. 书架增删改查
4. 阅读进度同步
5. 用户偏好设置同步
6. 读书笔记 CRUD
7. 基础书名检索

### P1

1. 多设备增量同步
2. 阅读时长统计
3. 书名规范化任务队列
4. 外部书籍信息聚合

### P2

1. 检索排序优化
2. 书名人工纠错后台
3. 推荐与个性化分析

## 17. 后续可扩展点

1. 拆分为独立的检索服务和同步服务
2. 引入 Elasticsearch / OpenSearch 做书名召回
3. 增加后台运营系统处理书籍去重与人工校验
4. 增加事件总线，沉淀用户阅读行为数据

## 18. 结论

这个项目最适合从“模块化单体”启动：

- Go 负责 API、同步编排和业务实现
- MySQL 承载核心业务数据
- Redis 承载缓存、会话和异步任务
- 书籍主数据、用户阅读数据、同步日志、规范化任务分层建模

这样既能快速支撑 `gravity-reader` 的首期需求，又能为后续多端同步、检索增强和服务拆分预留清晰演进路径。

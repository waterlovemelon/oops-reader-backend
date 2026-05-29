# Oops Reader Community Persistence Design

Date: 2026-05-30

## 1. 背景

当前 `internal/community` 使用 `Service` 内部的内存 map 保存 boards、threads、comments 和 reactions。API 进程重启后，书友区帖子、评论、回复和点赞都会丢失。

社区模块已经有基础 HTTP 接口：

- `GET /v1/community/boards`
- `GET /v1/community/threads`
- `POST /v1/community/threads`
- `GET /v1/community/threads/:id`
- `POST /v1/community/threads/:id/comments`
- `POST /v1/community/reactions`

本设计将社区相关内容迁移到 MariaDB/MySQL 持久化，同时为帖子图片、评论图片、审核、举报、置顶、收藏和后续通知等功能保留扩展点。

## 2. 目标

1. 持久化社区 boards、threads、comments 和 reactions。
2. 保持现有 HTTP API 和响应结构基本兼容。
3. 使用和 `identity`、`catalog`、`backup` 一致的 Store 接口模式。
4. 支持无数据库测试场景，通过 MemoryStore 继续跑集成测试。
5. 为帖子和评论图片提供可扩展的附件模型。
6. 避免把图片二进制直接写入 MySQL。
7. 预留审核、软删除、置顶、举报、评论分页、对象存储和 CDN 的扩展空间。

## 3. 非目标

1. 第一阶段不实现完整后台管理系统。
2. 第一阶段不实现图片内容审核服务，只保留状态字段和表结构扩展点。
3. 第一阶段不实现全文搜索。后续可接入 MySQL FULLTEXT 或独立搜索服务。
4. 第一阶段不实现消息通知投递，只保证评论和反应事件具备可扩展的数据边界。
5. 第一阶段不实现对象存储厂商适配的全部实现，但接口需要允许后续接入 S3、OSS 或 COS。

## 4. 推荐方案

采用 `community.Store` 接口 + `MySQLStore` + `MemoryStore`：

- `Service` 负责输入校验、业务规则、ID 生成、时间注入、错误语义和事务级业务编排。
- `Store` 负责数据读写、SQL 扫描、事务实现和数据库错误转换。
- `cmd/api/main.go` 根据 `db == nil` 选择 `MemoryStore` 或 `MySQLStore`。

这个方案与现有代码风格一致。`identity`、`catalog` 和 `backup` 已经采用类似模式，测试可以继续使用内存实现，生产环境使用 MySQL。

不建议把 SQL 直接写进 `Service`。那会让业务校验、SQL、事务、数据扫描混在一起，后续添加审核、举报、图片、编辑历史和通知时维护成本会迅速升高。

## 5. 数据模型

### 5.1 Boards

`community_boards` 保存社区板块。默认板块通过迁移初始化，服务启动时也可以执行幂等的默认板块补齐。

```sql
CREATE TABLE community_boards (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  description VARCHAR(500) NULL,
  status ENUM('active', 'archived', 'deleted') NOT NULL DEFAULT 'active',
  sort_order INT NOT NULL DEFAULT 0,
  metadata JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_boards_status_sort (status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

默认数据：

- `general`
- `book-club`
- `help`

### 5.2 Threads

`community_threads` 保存帖子主数据。

```sql
CREATE TABLE community_threads (
  id VARCHAR(64) PRIMARY KEY,
  board_id VARCHAR(64) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(200) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  optional_book_id VARCHAR(191) NULL,
  status ENUM('active', 'hidden', 'locked', 'deleted') NOT NULL DEFAULT 'active',
  comment_count INT UNSIGNED NOT NULL DEFAULT 0,
  reaction_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_commented_at DATETIME NULL,
  metadata JSON NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  KEY idx_threads_board_status_updated (board_id, status, updated_at),
  KEY idx_threads_book (optional_book_id),
  KEY idx_threads_user_created (user_id, created_at),
  CONSTRAINT fk_threads_board FOREIGN KEY (board_id) REFERENCES community_boards(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

`status` 预留 `hidden` 和 `locked`，便于后续审核隐藏和锁帖。`metadata` 用于保存客户端版本、来源、审核摘要、章节定位等低频扩展信息。

### 5.3 Comments

`community_comments` 保存评论和回复。回复通过 `parent_comment_id` 表达，第一阶段允许一层或多层回复，业务层可以限制展示层级。

```sql
CREATE TABLE community_comments (
  id VARCHAR(64) PRIMARY KEY,
  thread_id VARCHAR(64) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  parent_comment_id VARCHAR(64) NULL,
  content TEXT NOT NULL,
  status ENUM('active', 'hidden', 'deleted') NOT NULL DEFAULT 'active',
  reaction_count INT UNSIGNED NOT NULL DEFAULT 0,
  metadata JSON NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  KEY idx_comments_thread_status_created (thread_id, status, created_at),
  KEY idx_comments_parent (parent_comment_id),
  KEY idx_comments_user_created (user_id, created_at),
  CONSTRAINT fk_comments_thread FOREIGN KEY (thread_id) REFERENCES community_threads(id),
  CONSTRAINT fk_comments_parent FOREIGN KEY (parent_comment_id) REFERENCES community_comments(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### 5.4 Reactions

`community_reactions` 保存用户对帖子或评论的反应。当前支持 `like` 和 `love`，后续可通过 `reaction_type` 扩展。

```sql
CREATE TABLE community_reactions (
  id VARCHAR(64) PRIMARY KEY,
  user_id BIGINT UNSIGNED NOT NULL,
  target_type ENUM('thread', 'comment') NOT NULL,
  target_id VARCHAR(64) NOT NULL,
  reaction_type VARCHAR(32) NOT NULL,
  status ENUM('active', 'deleted') NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE KEY uniq_reactions_user_target (user_id, target_type, target_id),
  KEY idx_reactions_target_status (target_type, target_id, status),
  KEY idx_reactions_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

同一用户对同一个目标只能有一个当前反应。再次反应时使用 upsert 更新 `reaction_type` 和 `updated_at`。

### 5.5 Attachments

帖子图片和评论图片不直接写入帖子表，也不使用 MySQL BLOB。图片作为独立附件资源保存，MySQL 只保存元数据和归属关系。

```sql
CREATE TABLE community_attachments (
  id VARCHAR(64) PRIMARY KEY,
  owner_user_id BIGINT UNSIGNED NOT NULL,
  target_type ENUM('thread', 'comment') NULL,
  target_id VARCHAR(64) NULL,
  file_type ENUM('image') NOT NULL,
  storage_provider VARCHAR(32) NOT NULL,
  storage_key VARCHAR(512) NOT NULL,
  public_url VARCHAR(1024) NULL,
  mime_type VARCHAR(100) NOT NULL,
  file_size INT UNSIGNED NOT NULL,
  width INT UNSIGNED NULL,
  height INT UNSIGNED NULL,
  checksum_sha256 CHAR(64) NOT NULL,
  status ENUM('pending', 'active', 'deleted', 'rejected') NOT NULL DEFAULT 'pending',
  sort_order INT NOT NULL DEFAULT 0,
  metadata JSON NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE KEY uniq_attachments_storage_key (storage_provider, storage_key),
  KEY idx_attachments_owner_status (owner_user_id, status, created_at),
  KEY idx_attachments_target (target_type, target_id, status, sort_order),
  KEY idx_attachments_checksum (checksum_sha256)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

附件状态含义：

- `pending`: 已上传但未绑定到帖子或评论。
- `active`: 已绑定且可展示。
- `deleted`: 用户或系统删除。
- `rejected`: 后续审核拒绝。

## 6. 图片存储和处理

### 6.1 存储抽象

新增附件存储接口：

```go
type AttachmentStorage interface {
    Save(ctx context.Context, input SaveAttachmentInput) (StoredAttachment, error)
    URL(ctx context.Context, key string) (string, error)
    Delete(ctx context.Context, key string) error
}
```

第一阶段实现 `LocalStorage`，通过配置指定根目录，例如 `/data/oops-reader/community/images`。后续可增加 `S3Storage`、`OSSStorage` 或 `COSStorage`，业务表保持不变。

### 6.2 上传流程

采用两阶段上传和绑定：

1. 客户端调用 `POST /v1/community/attachments` 上传图片。
2. 服务端校验 MIME、大小、尺寸，计算 SHA-256，保存文件。
3. 服务端写入 `community_attachments`，状态为 `pending`，返回 `attachment_id`。
4. 客户端发帖或评论时携带 `attachment_ids`。
5. 创建帖子或评论的事务内校验附件归属当前用户、状态为 `pending`、尚未绑定。
6. 事务内将附件绑定到 `thread` 或 `comment`，状态改为 `active`。

这个流程避免发帖接口同时处理大文件，也方便后续图片复用到评论、举报证据和其他社区内容。

### 6.3 校验规则

第一阶段建议规则：

- 单图最大 5 MB。
- 单帖最多 9 张图片。
- 单条评论最多 3 张图片。
- 允许 `image/jpeg`、`image/png`、`image/webp`。
- 使用文件头检测 MIME，不信任客户端文件名。
- 文件名由服务端生成，禁止使用用户上传文件名作为存储路径。
- 保存宽高，便于前端提前布局。
- 保存 SHA-256，便于去重、安全审计和排查。

### 6.4 图片变体

第一阶段可以只保存原图元数据。后续需要缩略图或压缩图时，有两个扩展方式：

1. 在 `metadata` 中保存 variants：

```json
{
  "variants": {
    "thumb": "community/images/2026/05/att_xxx_thumb.webp",
    "large": "community/images/2026/05/att_xxx_large.webp"
  }
}
```

2. 后续新增 `community_attachment_variants` 表，适合变体类型变多、需要独立状态和尺寸字段时使用。

第一阶段优先使用 `metadata`，避免过早扩表。

## 7. Go 模块设计

### 7.1 Service 构造

社区服务改为依赖 Store：

```go
type Service struct {
    store Store
    now   func() time.Time
}

func NewService(store Store) *Service
func NewServiceWithDB(db *sql.DB) *Service
```

`NewServiceWithDB(nil)` 使用 `MemoryStore`，有数据库时使用 `MySQLStore`。

### 7.2 Store 接口

建议接口按当前功能和附件能力拆分：

```go
type Store interface {
    ListBoards(ctx context.Context) ([]Board, error)
    EnsureDefaultBoards(ctx context.Context, boards []Board) error

    ListThreads(ctx context.Context, boardID string, limit, offset int) ([]Thread, error)
    CreateThread(ctx context.Context, thread Thread, attachmentIDs []string) (Thread, error)
    GetThread(ctx context.Context, id string) (Thread, error)

    ListComments(ctx context.Context, threadID string) ([]Comment, error)
    CreateComment(ctx context.Context, comment Comment, attachmentIDs []string) (Comment, error)

    UpsertReaction(ctx context.Context, reaction Reaction) (Reaction, error)
    CountReactions(ctx context.Context, targets []ReactionTarget) (map[ReactionTarget]map[string]int, error)

    CreateAttachment(ctx context.Context, attachment Attachment) (Attachment, error)
    BindAttachments(ctx context.Context, ownerUserID, targetType, targetID string, attachmentIDs []string) error
    ListAttachments(ctx context.Context, targets []AttachmentTarget) (map[AttachmentTarget][]Attachment, error)
}
```

`CreateThread` 和 `CreateComment` 在 MySQL 实现中应使用事务，保证主内容和附件绑定同时成功或同时失败。

### 7.3 数据结构

保留现有 `Board`、`Thread`、`Comment` 和 `Reaction`，增加附件结构：

```go
type Attachment struct {
    ID              string
    OwnerUserID     string
    TargetType      string
    TargetID        string
    FileType        string
    StorageProvider string
    StorageKey      string
    PublicURL       string
    MIMEType        string
    FileSize        uint
    Width           uint
    Height          uint
    ChecksumSHA256  string
    Status          string
    SortOrder       int
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

`Thread` 和 `Comment` 增加：

```go
Attachments []Attachment
```

### 7.4 错误处理

沿用当前 sentinel error 风格：

- `ErrInvalidInput`
- `ErrBoardNotFound`
- `ErrThreadNotFound`
- `ErrCommentNotFound`
- `ErrAttachmentNotFound`
- `ErrAttachmentNotOwned`
- `ErrAttachmentAlreadyBound`
- `ErrAttachmentLimitExceeded`

HTTP handler 使用 `errors.Is()` 转换为状态码。

## 8. API 设计

### 8.1 兼容现有接口

现有接口路径保持不变。帖子和评论响应新增 `attachments` 字段，不影响旧客户端读取原字段。

线程响应示例：

```json
{
  "data": {
    "id": "thr_xxx",
    "board_id": "general",
    "user_id": "1",
    "title": "阅读感想",
    "content": "这本书适合慢慢读。",
    "optional_book_id": "book-1",
    "status": "active",
    "comment_count": 2,
    "reaction_counts": {
      "like": 3
    },
    "attachments": [
      {
        "id": "att_xxx",
        "file_type": "image",
        "url": "https://example.com/community/images/att_xxx.webp",
        "mime_type": "image/webp",
        "width": 1200,
        "height": 800
      }
    ],
    "comments": []
  }
}
```

### 8.2 上传附件

新增接口：

`POST /v1/community/attachments`

要求登录。请求使用 `multipart/form-data`，字段名为 `file`。

响应：

```json
{
  "data": {
    "id": "att_xxx",
    "file_type": "image",
    "mime_type": "image/jpeg",
    "file_size": 123456,
    "width": 1200,
    "height": 800,
    "status": "pending"
  }
}
```

### 8.3 发帖携带附件

`POST /v1/community/threads` 请求增加可选字段：

```json
{
  "board_id": "general",
  "title": "阅读感想",
  "content": "这本书适合慢慢读。",
  "optional_book_id": "book-1",
  "attachment_ids": ["att_xxx"]
}
```

### 8.4 评论携带附件

`POST /v1/community/threads/:id/comments` 请求增加可选字段：

```json
{
  "parent_comment_id": "",
  "content": "我也这么觉得。",
  "attachment_ids": ["att_yyy"]
}
```

### 8.5 后续评论分页

当前 `GetThread` 可以继续返回全量评论。评论量增长后新增：

`GET /v1/community/threads/:id/comments?page=1&page_size=20`

旧接口保留，避免破坏客户端。

## 9. 数据流

### 9.1 发帖

1. Handler 从 auth middleware 读取 `user_id`。
2. Handler 绑定 JSON 请求。
3. Service 校验 board、title、content 和附件数量。
4. Service 生成 thread ID 和时间戳。
5. Store 开启事务。
6. Store 插入 `community_threads`。
7. Store 校验并绑定附件。
8. Store 提交事务。
9. Service 查询 reaction counts、attachments，返回完整 Thread。

### 9.2 评论

1. Handler 读取登录用户。
2. Service 校验 thread 存在且 active。
3. 如果有 `parent_comment_id`，校验父评论属于同一 thread 且 active。
4. Store 事务内插入 comment。
5. Store 更新 thread 的 `comment_count`、`updated_at` 和 `last_commented_at`。
6. Store 绑定附件。
7. 返回新评论。

### 9.3 点赞或喜欢

1. Service 校验 target type、target ID 和 reaction type。
2. Store 校验目标存在且 active。
3. Store 使用 `INSERT ... ON DUPLICATE KEY UPDATE` 写入 reaction。
4. Store 更新目标的 `reaction_count`。计数可第一阶段即时更新，也可后续通过聚合查询修正。

## 10. 迁移策略

1. 新增 `migrations/009_init_community.sql`。
2. 插入默认 boards。
3. 增加 `internal/community/store.go`。
4. 增加 `internal/community/memory_store.go`，保持测试可用。
5. 增加 `internal/community/mysql_store.go`。
6. 改造 `internal/community/service.go`，从 map 存储迁移到 Store。
7. 改造 `internal/transport/http/handlers/community.go`，增加附件字段和上传接口。
8. 改造 `cmd/api/main.go`，根据 DB 注入社区 Store。
9. 增加配置项控制本地附件目录和上传限制。
10. 增加单元测试和 HTTP 集成测试。

## 11. 测试计划

### 11.1 Service 测试

覆盖：

- 默认 boards 可查询。
- 创建帖子后可列表和详情读取。
- 创建评论后 `comment_count` 更新。
- 回复不存在的评论返回 `ErrCommentNotFound`。
- 同一用户重复反应会更新 reaction type，而不是新增多条有效反应。
- 附件不属于当前用户时不能绑定。
- 超出附件数量限制返回 `ErrAttachmentLimitExceeded`。

### 11.2 Store 测试

如果测试环境可用 MySQL，增加 MySQLStore 集成测试：

- 事务失败时 thread/comment 和附件绑定都回滚。
- reaction unique key 生效。
- 软删除内容不会出现在普通查询结果中。

无 MySQL 时至少保证 MemoryStore 覆盖同一套 Service 行为测试。

### 11.3 HTTP 测试

覆盖：

- 未登录不能发帖、评论、上传附件和反应。
- 登录后可以发帖、评论和反应。
- 旧的 boards、threads、thread detail 响应结构仍保留。
- 发帖带 `attachment_ids` 时响应返回 `attachments`。

## 12. 扩展方向

### 12.1 审核和举报

后续新增：

- `community_reports`
- `community_moderation_actions`

帖子和评论已有 `hidden`、`deleted` 状态，可直接承接审核结果。

### 12.2 置顶和精华

简单实现可以在 `community_threads.metadata` 保存标记。正式实现建议增加：

- `pinned_at`
- `featured_at`
- `moderated_by`

如果置顶规则按板块、时间窗口和管理员动作变化较多，可以单独建 `community_thread_flags`。

### 12.3 编辑历史

后续新增：

- `community_thread_edits`
- `community_comment_edits`

主表只保存当前版本，历史表保存编辑前后的标题、正文、操作者和时间。

### 12.4 通知

评论和回复创建后，可以由 Service 或后续事件层写入通知模块。第一阶段不强制引入事件流，但 Store 边界应避免把通知逻辑写进 MySQLStore。

### 12.5 搜索

第一阶段用普通索引和分页。后续可以选择：

- MySQL FULLTEXT，用于简单标题和正文搜索。
- 独立搜索服务，用于更复杂的中文分词、排序和高亮。

## 13. 实施顺序

推荐分三步落地：

1. **基础持久化**：boards、threads、comments、reactions 的 Store 化和 MySQL 迁移。
2. **附件元数据和本地图片存储**：上传接口、附件表、LocalStorage、发帖和评论绑定附件。
3. **扩展能力补齐**：评论分页、软删除接口、审核状态流转、对象存储实现和图片变体。

这样可以先解决重启丢数据问题，再引入图片上传，最后补齐社区长期运营能力。

-- 009_init_community.sql
-- Community persistence: boards, threads, comments, reactions, attachments

CREATE TABLE IF NOT EXISTS community_boards (
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

INSERT IGNORE INTO community_boards (id, name, description, sort_order) VALUES
  ('general', 'General', 'Open discussion for readers.', 1),
  ('book-club', 'Book Club', 'Shared reads, prompts, and group notes.', 2),
  ('help', 'Help', 'Questions about books, imports, and the app.', 3);

CREATE TABLE IF NOT EXISTS community_threads (
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

CREATE TABLE IF NOT EXISTS community_comments (
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

CREATE TABLE IF NOT EXISTS community_reactions (
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

CREATE TABLE IF NOT EXISTS community_attachments (
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

-- Sync and Normalization Tables
-- Date: 2025-03-16
-- Description: Client sync operations, cursors, title normalization tasks, and search logs

-- 11.1 sync_operations table
-- Client sync operation log table supporting idempotency and audit
CREATE TABLE IF NOT EXISTS `sync_operations` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `device_id` VARCHAR(128) NOT NULL COMMENT 'Device ID',
    `operation_id` VARCHAR(128) NOT NULL COMMENT 'Client operation unique ID',
    `entity_type` VARCHAR(32) NOT NULL COMMENT 'bookshelf/progress/note/preference',
    `entity_id` VARCHAR(128) NOT NULL COMMENT 'Entity identifier',
    `operation_type` VARCHAR(32) NOT NULL COMMENT 'create/update/delete',
    `payload` JSON NOT NULL COMMENT 'Operation content',
    `client_occurred_at` DATETIME NOT NULL COMMENT 'Client occurrence time',
    `server_version` BIGINT UNSIGNED NOT NULL COMMENT 'Server version number',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    UNIQUE KEY `uk_user_device_op` (`user_id`, `device_id`, `operation_id`),
    INDEX `idx_user_version` (`user_id`, `server_version`),
    INDEX `idx_user_entity` (`user_id`, `entity_type`, `entity_id`),
    CONSTRAINT `fk_sync_operations_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Client sync operation log table';

-- 11.2 sync_cursors table
-- Each user or device sync cursor
CREATE TABLE IF NOT EXISTS `sync_cursors` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `device_id` VARCHAR(128) NOT NULL COMMENT 'Device ID',
    `last_pulled_version` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Last pulled version',
    `last_pushed_at` DATETIME NULL COMMENT 'Last pushed time',
    `last_pulled_at` DATETIME NULL COMMENT 'Last pulled time',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_user_device` (`user_id`, `device_id`),
    CONSTRAINT `fk_sync_cursors_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Sync cursor table';

-- 11.3 book_title_normalization_tasks table
-- Book title normalization task table
CREATE TABLE IF NOT EXISTS `book_title_normalization_tasks` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NULL COMMENT 'Source user ID',
    `raw_title` VARCHAR(255) NOT NULL COMMENT 'Original book title input',
    `normalized_title` VARCHAR(255) NOT NULL COMMENT 'Cleaned title',
    `matched_book_id` BIGINT UNSIGNED NULL COMMENT 'Matched standard book ID',
    `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0=Pending, 1=Matched, 2=Pending confirmation, 3=Ignored',
    `confidence_score` DECIMAL(5,2) NOT NULL DEFAULT 0 COMMENT 'Match confidence score',
    `source` VARCHAR(32) NOT NULL COMMENT 'search/import/manual',
    `context_payload` JSON NULL COMMENT 'Context information',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    INDEX `idx_normalized_title` (`normalized_title`),
    INDEX `idx_status_created_at` (`status`, `created_at`),
    INDEX `idx_matched_book_id` (`matched_book_id`),
    CONSTRAINT `fk_book_title_normalization_tasks_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL,
    CONSTRAINT `fk_book_title_normalization_tasks_book_id` FOREIGN KEY (`matched_book_id`) REFERENCES `books`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Book title normalization task table';

-- 11.4 book_search_logs table
-- Book search log table for optimizing recall and ranking
CREATE TABLE IF NOT EXISTS `book_search_logs` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NULL COMMENT 'User ID',
    `query_text` VARCHAR(255) NOT NULL COMMENT 'Original query term',
    `normalized_query` VARCHAR(255) NOT NULL COMMENT 'Normalized query term',
    `result_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Result count',
    `selected_book_id` BIGINT UNSIGNED NULL COMMENT 'Finally selected book',
    `source` VARCHAR(32) NOT NULL COMMENT 'local/mixed/external',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    INDEX `idx_normalized_query_created_at` (`normalized_query`, `created_at`),
    INDEX `idx_user_created_at` (`user_id`, `created_at`),
    CONSTRAINT `fk_book_search_logs_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL,
    CONSTRAINT `fk_book_search_logs_book_id` FOREIGN KEY (`selected_book_id`) REFERENCES `books`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Book search log table';

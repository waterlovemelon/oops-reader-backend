-- User Reading Data Tables
-- Date: 2025-03-16
-- Description: User bookshelves, reading progress, sessions, stats, notes, and preferences

-- 10.1 user_bookshelves table
-- User bookshelf table, core association between users and books
CREATE TABLE IF NOT EXISTS `user_bookshelves` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `book_id` BIGINT UNSIGNED NOT NULL COMMENT 'Book ID',
    `shelf_status` VARCHAR(32) NOT NULL COMMENT 'want_to_read/reading/finished/archived',
    `source_type` VARCHAR(32) NOT NULL DEFAULT 'manual' COMMENT 'manual/import/search',
    `rating` TINYINT UNSIGNED NULL COMMENT 'User rating 1-10',
    `started_at` DATETIME NULL COMMENT 'Start reading time',
    `finished_at` DATETIME NULL COMMENT 'Finish reading time',
    `last_read_at` DATETIME NULL COMMENT 'Last read time',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    `deleted_at` DATETIME NULL COMMENT 'Soft delete time',
    UNIQUE KEY `uk_user_book` (`user_id`, `book_id`),
    INDEX `idx_user_status_last_read` (`user_id`, `shelf_status`, `last_read_at`),
    INDEX `idx_book_id` (`book_id`),
    CONSTRAINT `fk_user_bookshelves_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_user_bookshelves_book_id` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='User bookshelf table';

-- 10.2 reading_progress table
-- Reading progress table
CREATE TABLE IF NOT EXISTS `reading_progress` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `book_id` BIGINT UNSIGNED NOT NULL COMMENT 'Book ID',
    `progress_type` VARCHAR(32) NOT NULL COMMENT 'page/chapter/percent/location',
    `progress_value` VARCHAR(64) NOT NULL COMMENT 'Progress value',
    `progress_percent` DECIMAL(5,2) NULL COMMENT 'Percentage progress',
    `chapter_title` VARCHAR(255) NULL COMMENT 'Current chapter title',
    `position_cfi` VARCHAR(1024) NULL COMMENT 'EPUB position info',
    `updated_by_device_id` VARCHAR(128) NULL COMMENT 'Update device',
    `recorded_at` DATETIME NOT NULL COMMENT 'Progress generation time',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_user_book` (`user_id`, `book_id`),
    INDEX `idx_user_recorded_at` (`user_id`, `recorded_at`),
    CONSTRAINT `fk_reading_progress_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_reading_progress_book_id` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Reading progress table';

-- 10.3 reading_sessions table
-- Reading session detail table
CREATE TABLE IF NOT EXISTS `reading_sessions` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `book_id` BIGINT UNSIGNED NOT NULL COMMENT 'Book ID',
    `device_id` VARCHAR(128) NULL COMMENT 'Device ID',
    `started_at` DATETIME NOT NULL COMMENT 'Session start time',
    `ended_at` DATETIME NOT NULL COMMENT 'Session end time',
    `duration_seconds` INT UNSIGNED NOT NULL COMMENT 'Reading seconds',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    INDEX `idx_user_started_at` (`user_id`, `started_at`),
    INDEX `idx_user_book_started_at` (`user_id`, `book_id`, `started_at`),
    CONSTRAINT `fk_reading_sessions_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_reading_sessions_book_id` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Reading session detail table';

-- 10.4 reading_daily_stats table
-- User reading daily statistics table for fast queries
CREATE TABLE IF NOT EXISTS `reading_daily_stats` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `stat_date` DATE NOT NULL COMMENT 'Statistics date',
    `total_reading_seconds` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Total reading duration',
    `total_books_read` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Books read that day',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_user_date` (`user_id`, `stat_date`),
    CONSTRAINT `fk_reading_daily_stats_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='User reading daily statistics table';

-- 10.5 user_notes table
-- Book notes table
CREATE TABLE IF NOT EXISTS `user_notes` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `book_id` BIGINT UNSIGNED NOT NULL COMMENT 'Book ID',
    `note_type` VARCHAR(32) NOT NULL COMMENT 'note/highlight/thought',
    `quote_text` TEXT NULL COMMENT 'Quote content',
    `note_text` TEXT NULL COMMENT 'Note content',
    `chapter_title` VARCHAR(255) NULL COMMENT 'Chapter name',
    `position_ref` VARCHAR(1024) NULL COMMENT 'Position reference',
    `color` VARCHAR(32) NULL COMMENT 'Highlight color',
    `created_by_device_id` VARCHAR(128) NULL COMMENT 'Device ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    `deleted_at` DATETIME NULL COMMENT 'Soft delete time',
    INDEX `idx_user_book_created_at` (`user_id`, `book_id`, `created_at`),
    INDEX `idx_user_created_at` (`user_id`, `created_at`),
    CONSTRAINT `fk_user_notes_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_user_notes_book_id` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Book notes table';

-- 10.6 user_preferences table
-- User preference settings table
CREATE TABLE IF NOT EXISTS `user_preferences` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `theme` VARCHAR(32) NULL COMMENT 'light/dark/sepia',
    `font_family` VARCHAR(64) NULL COMMENT 'Font family',
    `font_size` INT UNSIGNED NULL COMMENT 'Font size',
    `line_height` DECIMAL(4,2) NULL COMMENT 'Line height',
    `page_animation` VARCHAR(32) NULL COMMENT 'Page turn animation',
    `sync_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT 'Whether enabled sync',
    `push_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT 'Whether enabled push',
    `extra_settings` JSON NULL COMMENT 'Extended settings',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_user_id` (`user_id`),
    CONSTRAINT `fk_user_preferences_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='User preference settings table';

-- User bookshelf and comments for online catalog books.
-- Date: 2026-06-01

CREATE TABLE IF NOT EXISTS `user_catalog_bookshelves` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `user_id` BIGINT UNSIGNED NOT NULL,
    `catalog_book_key` VARCHAR(191) NOT NULL,
    `shelf_status` VARCHAR(32) NOT NULL DEFAULT 'want_to_read',
    `local_book_id` VARCHAR(191) NULL,
    `added_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `last_read_at` DATETIME NULL,
    `deleted_at` DATETIME NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY `uk_user_catalog_book` (`user_id`, `catalog_book_key`),
    INDEX `idx_user_status_last_read` (`user_id`, `shelf_status`, `last_read_at`),
    INDEX `idx_catalog_book_key` (`catalog_book_key`),
    CONSTRAINT `fk_user_catalog_bookshelves_user`
        FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='User bookshelf records for online catalog books';

CREATE TABLE IF NOT EXISTS `catalog_book_comments` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `comment_id` VARCHAR(191) NOT NULL,
    `catalog_book_key` VARCHAR(191) NOT NULL,
    `user_id` BIGINT UNSIGNED NULL,
    `source` VARCHAR(32) NOT NULL DEFAULT 'user',
    `external_source` VARCHAR(32) NULL,
    `external_id` VARCHAR(191) NULL,
    `author_name` VARCHAR(191) NULL,
    `content` TEXT NOT NULL,
    `like_count` INT UNSIGNED NOT NULL DEFAULT 0,
    `status` VARCHAR(32) NOT NULL DEFAULT 'published',
    `imported_at` DATETIME NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `deleted_at` DATETIME NULL,
    UNIQUE KEY `uk_comment_id` (`comment_id`),
    UNIQUE KEY `uk_external_comment` (`external_source`, `external_id`),
    INDEX `idx_book_status_created` (`catalog_book_key`, `status`, `created_at`),
    INDEX `idx_user_created` (`user_id`, `created_at`),
    CONSTRAINT `fk_catalog_book_comments_user`
        FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Reader comments for online catalog books';

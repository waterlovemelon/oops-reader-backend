-- Catalog EPUB asset index
-- Date: 2026-05-21
-- Description: Server-side catalog index for uploaded EPUB test books.

CREATE TABLE IF NOT EXISTS `catalog_books` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Catalog row ID',
    `book_id` BIGINT UNSIGNED NULL COMMENT 'Optional standard metadata book ID',
    `book_key` VARCHAR(191) NOT NULL COMMENT 'Stable public catalog identifier',
    `title` VARCHAR(255) NOT NULL COMMENT 'Display title',
    `author` VARCHAR(255) NULL COMMENT 'Display author',
    `filename` VARCHAR(255) NOT NULL COMMENT 'Original EPUB filename',
    `storage_path` VARCHAR(1024) NOT NULL COMMENT 'Absolute or catalog-root-relative EPUB path',
    `file_size` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'EPUB file size in bytes',
    `content_sha1` CHAR(40) NULL COMMENT 'SHA1 of EPUB content for duplicate detection',
    `language` VARCHAR(32) NULL COMMENT 'EPUB language if known',
    `chapter_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Parsed chapter count',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=Active, 2=Hidden, 3=Deleted',
    `indexed_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last index time',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_book_key` (`book_key`),
    INDEX `idx_content_sha1` (`content_sha1`),
    INDEX `idx_title` (`title`),
    INDEX `idx_author` (`author`),
    INDEX `idx_status_updated_at` (`status`, `updated_at`),
    INDEX `idx_book_id` (`book_id`),
    CONSTRAINT `fk_catalog_books_book_id` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Uploaded EPUB catalog index';

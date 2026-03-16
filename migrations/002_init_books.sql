-- Books and Metadata Tables
-- Date: 2025-03-16
-- Description: Standard book metadata, authors, and external source mappings

-- 9.1 books table
-- Standard book main table
CREATE TABLE IF NOT EXISTS `books` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Book ID',
    `canonical_title` VARCHAR(255) NOT NULL COMMENT 'Standard title',
    `subtitle` VARCHAR(255) NULL COMMENT 'Subtitle',
    `original_title` VARCHAR(255) NULL COMMENT 'Original title',
    `description` TEXT NULL COMMENT 'Description',
    `publisher` VARCHAR(128) NULL COMMENT 'Publisher',
    `publish_date` DATE NULL COMMENT 'Publication date',
    `isbn10` VARCHAR(16) NULL COMMENT 'ISBN-10',
    `isbn13` VARCHAR(20) NULL COMMENT 'ISBN-13',
    `page_count` INT UNSIGNED NULL COMMENT 'Page count',
    `language` VARCHAR(32) NULL COMMENT 'Language',
    `cover_url` VARCHAR(512) NULL COMMENT 'Cover URL',
    `source_confidence` DECIMAL(5,2) NOT NULL DEFAULT 0 COMMENT 'Metadata confidence score',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=Active, 2=Pending, 3=Deprecated',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_isbn13` (`isbn13`),
    UNIQUE KEY `uk_isbn10` (`isbn10`),
    INDEX `idx_title` (`canonical_title`),
    INDEX `idx_publisher_publish_date` (`publisher`, `publish_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Standard book main table';

-- 9.2 authors table
-- Author table
CREATE TABLE IF NOT EXISTS `authors` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Author ID',
    `name` VARCHAR(128) NOT NULL COMMENT 'Author name',
    `name_normalized` VARCHAR(128) NOT NULL COMMENT 'Normalized author name',
    `bio` TEXT NULL COMMENT 'Biography',
    `avatar_url` VARCHAR(512) NULL COMMENT 'Avatar URL',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    INDEX `idx_name_normalized` (`name_normalized`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Author table';

-- 9.3 book_authors table
-- Many-to-many relationship between books and authors
CREATE TABLE IF NOT EXISTS `book_authors` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `book_id` BIGINT UNSIGNED NOT NULL COMMENT 'Book ID',
    `author_id` BIGINT UNSIGNED NOT NULL COMMENT 'Author ID',
    `role` VARCHAR(32) NOT NULL DEFAULT 'author' COMMENT 'author/translator/editor',
    `sort_order` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Display order',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    UNIQUE KEY `uk_book_author_role` (`book_id`, `author_id`, `role`),
    INDEX `idx_author_id` (`author_id`),
    CONSTRAINT `fk_book_authors_book_id` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_book_authors_author_id` FOREIGN KEY (`author_id`) REFERENCES `authors`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Book-author relationship table';

-- 9.4 book_aliases table
-- Book title aliases for normalization matching
CREATE TABLE IF NOT EXISTS `book_aliases` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `book_id` BIGINT UNSIGNED NOT NULL COMMENT 'Standard book ID',
    `alias_title` VARCHAR(255) NOT NULL COMMENT 'Alias title',
    `alias_title_normalized` VARCHAR(255) NOT NULL COMMENT 'Normalized alias title',
    `alias_type` VARCHAR(32) NOT NULL COMMENT 'original/translated/common/user_input',
    `source` VARCHAR(32) NOT NULL COMMENT 'system/provider/manual',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    INDEX `idx_alias_title_normalized` (`alias_title_normalized`),
    INDEX `idx_book_id` (`book_id`),
    CONSTRAINT `fk_book_aliases_book_id` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Book title aliases table';

-- 9.5 book_external_sources table
-- External book source mapping table
CREATE TABLE IF NOT EXISTS `book_external_sources` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `book_id` BIGINT UNSIGNED NOT NULL COMMENT 'Local book ID',
    `source_name` VARCHAR(32) NOT NULL COMMENT 'douban/google_books/openlibrary etc.',
    `external_book_id` VARCHAR(128) NOT NULL COMMENT 'External book ID',
    `raw_payload` JSON NULL COMMENT 'Raw response data',
    `sync_status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=Active, 2=Pending update, 3=Inactive',
    `last_synced_at` DATETIME NULL COMMENT 'Last sync time',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_source_external_id` (`source_name`, `external_book_id`),
    INDEX `idx_book_id` (`book_id`),
    CONSTRAINT `fk_book_external_sources_book_id` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='External book source mapping table';

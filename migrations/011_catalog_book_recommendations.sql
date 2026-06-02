CREATE TABLE IF NOT EXISTS `catalog_book_recommendations` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `book_key` VARCHAR(191) NOT NULL,
    `comment` TEXT NOT NULL,
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `scheduled_publish_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_by` VARCHAR(191) NULL,
    `updated_by` VARCHAR(191) NULL,
    `deleted_at` DATETIME NULL,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_recommendations_status_publish` (`status`, `scheduled_publish_at`, `id`),
    INDEX `idx_recommendations_book` (`book_key`),
    INDEX `idx_recommendations_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Scheduled and historical recommended catalog books';

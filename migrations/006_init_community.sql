CREATE TABLE IF NOT EXISTS `community_boards` (
    `id` VARCHAR(64) NOT NULL PRIMARY KEY,
    `name` VARCHAR(80) NOT NULL,
    `description` VARCHAR(255) NOT NULL DEFAULT '',
    `sort_order` INT NOT NULL DEFAULT 0,
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_community_boards_status_sort` (`status`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO `community_boards` (`id`, `name`, `description`, `sort_order`, `status`)
VALUES
    ('general', '闲聊', '随便聊聊最近读到的书、句子和生活。', 10, 'active'),
    ('book-club', '共读小组', '一起读一本书，交换进度和问题。', 20, 'active'),
    ('help', '阅读帮助', '导入、排版、听书和阅读体验问题。', 30, 'active')
ON DUPLICATE KEY UPDATE
    `name` = VALUES(`name`),
    `description` = VALUES(`description`),
    `sort_order` = VALUES(`sort_order`),
    `status` = VALUES(`status`);

CREATE TABLE IF NOT EXISTS `community_threads` (
    `id` VARCHAR(64) NOT NULL PRIMARY KEY,
    `board_id` VARCHAR(64) NOT NULL,
    `author_user_id` VARCHAR(64) NOT NULL,
    `title` VARCHAR(120) NOT NULL,
    `content` TEXT NOT NULL,
    `optional_book_id` VARCHAR(191) NOT NULL DEFAULT '',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `comment_count` INT UNSIGNED NOT NULL DEFAULT 0,
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_community_threads_board_updated` (`board_id`, `status`, `updated_at`),
    INDEX `idx_community_threads_author_updated` (`author_user_id`, `status`, `updated_at`),
    CONSTRAINT `fk_community_threads_board_id` FOREIGN KEY (`board_id`) REFERENCES `community_boards` (`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `community_comments` (
    `id` VARCHAR(64) NOT NULL PRIMARY KEY,
    `thread_id` VARCHAR(64) NOT NULL,
    `author_user_id` VARCHAR(64) NOT NULL,
    `parent_comment_id` VARCHAR(64) NOT NULL DEFAULT '',
    `content` TEXT NOT NULL,
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_community_comments_thread_created` (`thread_id`, `status`, `created_at`),
    INDEX `idx_community_comments_parent` (`parent_comment_id`),
    CONSTRAINT `fk_community_comments_thread_id` FOREIGN KEY (`thread_id`) REFERENCES `community_threads` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `community_reactions` (
    `id` VARCHAR(64) NOT NULL PRIMARY KEY,
    `author_user_id` VARCHAR(64) NOT NULL,
    `target_type` VARCHAR(32) NOT NULL,
    `target_id` VARCHAR(64) NOT NULL,
    `reaction_type` VARCHAR(32) NOT NULL,
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY `uk_community_reactions_user_target` (`author_user_id`, `target_type`, `target_id`),
    INDEX `idx_community_reactions_target_status` (`target_type`, `target_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

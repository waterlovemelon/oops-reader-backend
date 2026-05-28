-- TTS user preferences table
-- Date: 2026-05-28
-- Description: Store per-user TTS provider and voice selection

CREATE TABLE IF NOT EXISTS `user_tts_prefs` (
    `user_id`    BIGINT UNSIGNED NOT NULL PRIMARY KEY COMMENT 'User ID',
    `provider`   VARCHAR(32) NOT NULL DEFAULT 'edge' COMMENT 'TTS provider name',
    `voice`      VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Selected voice identifier',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    CONSTRAINT `fk_user_tts_prefs_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='User TTS provider and voice preferences';

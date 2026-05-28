-- Account system tables and fields
-- Date: 2026-05-28
-- Description: Persistent account status, password reset, entitlements, and latest reading-data backup

ALTER TABLE `users`
    ADD COLUMN IF NOT EXISTS `account_status` VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/frozen/deactivated' AFTER `avatar_url`,
    ADD COLUMN IF NOT EXISTS `account_type` VARCHAR(32) NOT NULL DEFAULT 'normal' COMMENT 'normal, future vip' AFTER `account_status`,
    ADD COLUMN IF NOT EXISTS `email_verified_at` DATETIME NULL COMMENT 'Email verification time' AFTER `account_type`;

CREATE INDEX IF NOT EXISTS `idx_users_account_status` ON `users` (`account_status`);
CREATE INDEX IF NOT EXISTS `idx_users_account_type` ON `users` (`account_type`);

ALTER TABLE `user_sessions`
    ADD COLUMN IF NOT EXISTS `revoked_at` DATETIME NULL COMMENT 'Revocation time' AFTER `expires_at`;

CREATE TABLE IF NOT EXISTS `password_reset_tokens` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `email` VARCHAR(128) NOT NULL COMMENT 'Email used for reset',
    `token_hash` VARCHAR(255) NOT NULL COMMENT 'Reset token hash',
    `expires_at` DATETIME NOT NULL COMMENT 'Expiration time',
    `used_at` DATETIME NULL COMMENT 'Used time',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    UNIQUE KEY `uk_token_hash` (`token_hash`),
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_email_created_at` (`email`, `created_at`),
    INDEX `idx_expires_at` (`expires_at`),
    CONSTRAINT `fk_password_reset_tokens_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Password reset tokens';

CREATE TABLE IF NOT EXISTS `account_entitlements` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `entitlement_key` VARCHAR(64) NOT NULL COMMENT 'Capability key',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/inactive/revoked',
    `source` VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'system/purchase/admin/promo',
    `starts_at` DATETIME NULL COMMENT 'Start time',
    `expires_at` DATETIME NULL COMMENT 'Expiration time',
    `metadata` JSON NULL COMMENT 'Extended metadata',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_user_entitlement` (`user_id`, `entitlement_key`),
    INDEX `idx_user_status` (`user_id`, `status`),
    INDEX `idx_expires_at` (`expires_at`),
    CONSTRAINT `fk_account_entitlements_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Account entitlements';

CREATE TABLE IF NOT EXISTS `user_data_backups` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `schema_version` INT UNSIGNED NOT NULL COMMENT 'Snapshot schema version',
    `payload` JSON NOT NULL COMMENT 'Reading data snapshot',
    `book_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Book count',
    `note_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Note count',
    `progress_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Progress count',
    `preference_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Preference count',
    `source_device_id` VARCHAR(128) NOT NULL COMMENT 'Source device ID',
    `source_device_name` VARCHAR(128) NULL COMMENT 'Source device name',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    UNIQUE KEY `uk_user_id` (`user_id`),
    INDEX `idx_updated_at` (`updated_at`),
    CONSTRAINT `fk_user_data_backups_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Latest user reading data backup';

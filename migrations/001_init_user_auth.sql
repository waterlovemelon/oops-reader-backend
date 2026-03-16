-- Users and Authentication Tables
-- Date: 2025-03-16
-- Description: User accounts, authentication providers, and session management

-- 8.1 users table
-- User main table
CREATE TABLE IF NOT EXISTS `users` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'User ID',
    `email` VARCHAR(128) NULL UNIQUE COMMENT 'Email',
    `phone` VARCHAR(32) NULL UNIQUE COMMENT 'Phone number',
    `password_hash` VARCHAR(255) NULL COMMENT 'Password hash',
    `nickname` VARCHAR(64) NOT NULL COMMENT 'Nickname',
    `avatar_url` VARCHAR(512) NULL COMMENT 'Avatar URL',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=Normal, 2=Frozen, 3=Deactivated',
    `last_login_at` DATETIME NULL COMMENT 'Last login time',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    INDEX `uk_email` (`email`),
    INDEX `uk_phone` (`phone`),
    INDEX `idx_status_created_at` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='User main table';

-- 8.2 user_auth_providers table
-- Third-party login bindings
CREATE TABLE IF NOT EXISTS `user_auth_providers` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `provider` VARCHAR(32) NOT NULL COMMENT 'google/apple/wechat etc.',
    `provider_user_id` VARCHAR(128) NOT NULL COMMENT 'Third-party user ID',
    `provider_union_id` VARCHAR(128) NULL COMMENT 'Optional unified ID',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    INDEX `uk_provider_uid` (`provider`, `provider_user_id`),
    INDEX `idx_user_id` (`user_id`),
    CONSTRAINT `fk_user_auth_providers_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Third-party login bindings table';

-- 8.3 user_sessions table
-- Login session table
CREATE TABLE IF NOT EXISTS `user_sessions` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT 'Primary key',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'User ID',
    `device_id` VARCHAR(128) NOT NULL COMMENT 'Device ID',
    `device_name` VARCHAR(128) NULL COMMENT 'Device name',
    `platform` VARCHAR(32) NOT NULL COMMENT 'ios/android/web/mac etc.',
    `refresh_token_hash` VARCHAR(255) NOT NULL COMMENT 'Refresh token hash',
    `expires_at` DATETIME NOT NULL COMMENT 'Expiration time',
    `last_active_at` DATETIME NOT NULL COMMENT 'Last active time',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created time',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated time',
    INDEX `idx_user_id` (`user_id`),
    INDEX `idx_device_id` (`device_id`),
    INDEX `idx_expires_at` (`expires_at`),
    CONSTRAINT `fk_user_sessions_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Login session table';

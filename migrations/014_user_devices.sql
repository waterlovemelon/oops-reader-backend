-- 账号设备表:记录账号使用过的设备,为后续"限制设备数量"预留。
-- Date: 2026-09-14
--
-- 阅读进度里的 updated_by_device_id 指向这里的 device_id。设备在登录/注册时首次登记,
-- 之后每次上报进度只刷新 last_seen_at。数量限制可直接 COUNT(*) WHERE user_id = ?。

CREATE TABLE IF NOT EXISTS `user_devices` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户 ID',
    `device_id` VARCHAR(128) NOT NULL COMMENT '客户端设备唯一标识',
    `device_name` VARCHAR(128) NULL COMMENT '设备名称,展示用',
    `platform` VARCHAR(32) NULL COMMENT 'linux/android/ios/...',
    `first_seen_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '首次登记时间',
    `last_seen_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最近活动时间',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_user_device` (`user_id`, `device_id`),
    INDEX `idx_user_last_seen` (`user_id`, `last_seen_at`),
    CONSTRAINT `fk_user_devices_user_id` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账号设备表';

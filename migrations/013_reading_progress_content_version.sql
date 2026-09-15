-- 阅读进度定位器所属的内容版本。
-- Date: 2026-09-14
--
-- unit/chapter/block 类定位器只在同一 manifest 版本内有效：服务端重跑预处理后
-- content_version 变化，旧定位器不再指向同一段文字。客户端拿到定位器时先比对版本，
-- 不一致就只按 progress_percent 重新定位。

ALTER TABLE `reading_progress`
    ADD COLUMN `content_version` VARCHAR(191) NULL COMMENT '定位器所属内容版本（manifest.content_version）' AFTER `position_cfi`;

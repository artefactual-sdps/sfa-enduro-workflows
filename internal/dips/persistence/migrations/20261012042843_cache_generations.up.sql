-- Modify "dip" table
ALTER TABLE `dip` MODIFY COLUMN `status` enum('queued','in progress','done','failed','deleted') NOT NULL, ADD COLUMN `expires_at` timestamp NULL, ADD COLUMN `workflow_id` varchar(255) NOT NULL, ADD INDEX `dip_doc_key` (`doc_key` (64));

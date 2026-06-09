-- 企业架构持久化表（蓝图 §7.2）
-- AutoMigrate 也会创建，此脚本供手工部署参考

CREATE TABLE IF NOT EXISTS message_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  message_id VARCHAR(64) NOT NULL UNIQUE,
  task_id VARCHAR(64) NOT NULL,
  trace_id VARCHAR(64),
  topic VARCHAR(128),
  kind VARCHAR(32),
  name VARCHAR(64),
  from_agent VARCHAR(32),
  to_agent VARCHAR(32),
  attempt INT DEFAULT 0,
  status VARCHAR(32),
  payload_json LONGTEXT,
  metadata_json TEXT,
  created_at DATETIME,
  acked_at DATETIME NULL,
  failed_at DATETIME NULL,
  deleted_at DATETIME NULL,
  INDEX idx_task (task_id),
  INDEX idx_status (status)
);

CREATE TABLE IF NOT EXISTS event_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  task_id VARCHAR(64) NOT NULL,
  trace_id VARCHAR(64),
  event_type VARCHAR(64) NOT NULL,
  agent VARCHAR(32),
  payload_json LONGTEXT,
  sequence_no BIGINT,
  created_at DATETIME,
  INDEX idx_task_seq (task_id, sequence_no)
);

CREATE TABLE IF NOT EXISTS task_checkpoints (
  task_id VARCHAR(64) PRIMARY KEY,
  trace_id VARCHAR(64),
  current_agent VARCHAR(32),
  current_message_id VARCHAR(64),
  round INT DEFAULT 1,
  retry_count INT DEFAULT 0,
  checkpoint_json LONGTEXT,
  updated_at DATETIME,
  INDEX idx_updated (updated_at)
);

CREATE TABLE IF NOT EXISTS worker_leases (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  task_id VARCHAR(64) NOT NULL,
  agent VARCHAR(32) NOT NULL,
  worker_id VARCHAR(128),
  lease_token VARCHAR(64),
  expires_at DATETIME,
  updated_at DATETIME,
  UNIQUE KEY idx_task_agent (task_id, agent)
);

CREATE TABLE IF NOT EXISTS dead_letters (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  task_id VARCHAR(64),
  message_id VARCHAR(64),
  topic VARCHAR(128),
  agent VARCHAR(32),
  reason TEXT,
  payload_json LONGTEXT,
  attempt INT,
  created_at DATETIME,
  INDEX idx_task (task_id)
);

CREATE TABLE IF NOT EXISTS outbox_messages (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  task_id VARCHAR(64) NOT NULL,
  message_id VARCHAR(64) NOT NULL UNIQUE,
  topic VARCHAR(128) NOT NULL,
  payload_json LONGTEXT NOT NULL,
  status VARCHAR(32) DEFAULT 'pending',
  retry_count INT DEFAULT 0,
  published_at DATETIME NULL,
  created_at DATETIME,
  INDEX idx_status (status)
);

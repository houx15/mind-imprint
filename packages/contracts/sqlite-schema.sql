-- Physical schema for S2. NOT executed in Slice 1. Mirrors @mind-imprint/contracts types.
CREATE TABLE task (
  id TEXT PRIMARY KEY, title TEXT NOT NULL, seed TEXT,
  status TEXT NOT NULL, created_at TEXT NOT NULL, last_active_at TEXT
);
CREATE TABLE message (
  id TEXT PRIMARY KEY, task_id TEXT NOT NULL REFERENCES task(id),
  role TEXT NOT NULL, content TEXT, tool_call TEXT, created_at TEXT NOT NULL
);
CREATE TABLE card_instance (            -- the standard envelope (CardInstance)
  id TEXT PRIMARY KEY, card_id TEXT NOT NULL, task_id TEXT NOT NULL REFERENCES task(id),
  parent_node_id TEXT, status TEXT NOT NULL,
  field_values TEXT NOT NULL,           -- JSON
  event_trace TEXT NOT NULL,            -- JSON
  rubric_tags TEXT NOT NULL,            -- JSON
  created_at TEXT NOT NULL, completed_at TEXT
);
CREATE TABLE process_node (
  id TEXT PRIMARY KEY, task_id TEXT NOT NULL REFERENCES task(id),
  parent_id TEXT, type TEXT NOT NULL, label TEXT, ref_id TEXT, meta TEXT
);
CREATE TABLE evaluation (
  task_id TEXT NOT NULL REFERENCES task(id),
  rubric_scores TEXT NOT NULL, narrative TEXT, created_at TEXT NOT NULL
);

CREATE TYPE resource_type AS ENUM ('Pod','Service','Deployment','ConfigMap','Ingress');
CREATE TYPE user_type AS ENUM ('origin','oauth2');
CREATE TYPE user_status AS ENUM ('online','offline','delete');
CREATE TYPE user_role AS ENUM ('admin','manager','user');

-- group_list
CREATE TABLE group_list (
  g_id SERIAL PRIMARY KEY,
  group_name VARCHAR(100) NOT NULL,
  description TEXT,
  create_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- projects
CREATE TABLE projects (
  p_id SERIAL PRIMARY KEY,
  project_name VARCHAR(100) NOT NULL,
  description TEXT,
  g_id INTEGER NOT NULL REFERENCES group_list(g_id) ON DELETE CASCADE ON UPDATE CASCADE,
  create_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- config_file
CREATE TABLE config_files (
  cf_id SERIAL PRIMARY KEY,
  filename VARCHAR(200) NOT NULL,
  content VARCHAR(5000),
  project_id INTEGER NOT NULL REFERENCES projects(p_id) ON DELETE CASCADE ON UPDATE CASCADE,
  create_at TIMESTAMP DEFAULT NOW()
);

-- resource
CREATE TABLE resources (
  r_id SERIAL PRIMARY KEY,
  cf_id INTEGER NOT NULL REFERENCES config_files(cf_id) ON DELETE CASCADE ON UPDATE CASCADE,
  type resource_type NOT NULL,
  name VARCHAR(50) NOT NULL,
  parsed_yaml JSONB NOT NULL,
  description TEXT,
  create_at TIMESTAMP DEFAULT NOW()
);

-- users
CREATE TABLE users (
  u_id SERIAL PRIMARY KEY,
  username VARCHAR(50) NOT NULL UNIQUE,
  password VARCHAR(255) NOT NULL,
  email VARCHAR(100),
  full_name VARCHAR(50),
  type user_type NOT NULL DEFAULT 'origin',
  status user_status NOT NULL DEFAULT 'offline',
  create_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- user_group
CREATE TABLE user_group (
  u_id INTEGER NOT NULL,
  g_id INTEGER NOT NULL,
  role user_role NOT NULL DEFAULT 'user',
  create_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (u_id, g_id),
  FOREIGN KEY (u_id) REFERENCES users(u_id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (g_id) REFERENCES group_list(g_id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- audit_logs
CREATE TABLE audit_logs (
  id SERIAL PRIMARY KEY,
  user_id INT NOT NULL,
  action VARCHAR(20) NOT NULL,
  resource_type VARCHAR(50) NOT NULL,
  resource_id VARCHAR NOT NULL,
  old_data JSONB,
  new_data JSONB,
  ip_address VARCHAR(45),
  user_agent TEXT,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- jobs
CREATE TABLE jobs (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(u_id) ON DELETE CASCADE ON UPDATE CASCADE,
  name VARCHAR(100) NOT NULL,
  namespace VARCHAR(100) NOT NULL,
  image VARCHAR(255) NOT NULL,
  status VARCHAR(50) DEFAULT 'Pending',
  k8s_job_name VARCHAR(100) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- View: project_group_views
CREATE OR REPLACE VIEW project_group_views AS
SELECT
  g.g_id,
  g.group_name,
  COUNT(DISTINCT p.p_id) AS project_count,
  COUNT(r.r_id) AS resource_count,
  MAX(g.create_at) AS group_create_at,
  MAX(g.update_at) AS group_update_at
FROM group_list g
LEFT JOIN projects p ON p.g_id = g.g_id
LEFT JOIN config_files cf ON cf.project_id = p.p_id
LEFT JOIN resources r ON r.cf_id = cf.cf_id
GROUP BY g.g_id, g.group_name;

-- View: project_resource_views
CREATE OR REPLACE VIEW project_resource_views AS
SELECT
  p.p_id,
  p.project_name,
  r.r_id,
  r.type,
  r.name,
  cf.filename,
  r.create_at AS resource_create_at
FROM projects p
JOIN config_files cf ON cf.project_id = p.p_id
JOIN resources r ON r.cf_id = cf.cf_id;

-- View: group_resource_views
CREATE OR REPLACE VIEW group_resource_views AS
SELECT
  g.g_id,
  g.group_name,
  p.p_id,
  p.project_name,
  r.r_id,
  r.type AS resource_type,
  r.name AS resource_name,
  cf.filename,
  r.create_at AS resource_create_at
FROM group_list g
LEFT JOIN projects p ON p.g_id = g.g_id
LEFT JOIN config_files cf ON cf.project_id = p.p_id
LEFT JOIN resources r ON r.cf_id = cf.cf_id
WHERE r.r_id IS NOT NULL;

-- View: user_group_views
CREATE OR REPLACE VIEW user_group_views AS
SELECT
  u.u_id,
  u.username,
  g.g_id,
  g.group_name,
  ug.role
FROM users u
JOIN user_group ug ON u.u_id = ug.u_id
JOIN group_list g ON ug.g_id = g.g_id;

-- View: users_with_superadmin
CREATE OR REPLACE VIEW users_with_superadmin AS
SELECT
  u.u_id,
  u.username,
  u.password,
  u.email,
  u.full_name,
  u.type,
  u.status,
  u.create_at,
  u.update_at,
  CASE WHEN ug.role = 'admin' AND ug.group_name = 'super' THEN true ELSE false END AS is_super_admin
FROM users u
LEFT JOIN user_group_views ug ON u.u_id = ug.u_id AND ug.group_name = 'super' AND ug.role = 'admin';

-- View: project_user_views

CREATE OR REPLACE VIEW project_user_views AS
SELECT
  p.p_id,
  p.project_name,
  g.g_id,
  g.group_name,
  u.u_id,
  u.username
FROM projects p
JOIN group_list g ON p.g_id = g.g_id
JOIN user_group ug ON ug.g_id = g.g_id
JOIN users u ON u.u_id = ug.u_id;

-- Initialize Data
INSERT INTO group_list (group_name, description)
VALUES ('super', 'Super administrator group')
ON CONFLICT DO NOTHING;

INSERT INTO users (username, password, type, status, full_name, email)
VALUES ('admin', '$2a$10$nsXJXOUAbVyLbvtPizj0RectJWdInu17C2NpWEVKNvwzKQcg8bchu', 'origin', 'offline', 'Administrator', '')
ON CONFLICT DO NOTHING;

INSERT INTO user_group (u_id, g_id, role)
SELECT u.u_id, g.g_id, 'admin'
FROM users u, group_list g
WHERE u.username = 'admin' AND g.group_name = 'super';

-- -- Uncomment to use pg_cron for log cleanup, ensure extension is installed
-- CREATE EXTENSION pg_cron;
-- SELECT cron.schedule(
--   'clear_audit_logs',
--   '0 3 * * *',
--   $$DELETE FROM audit_logs WHERE created_at < NOW() - INTERVAL '30 days'$$
-- );

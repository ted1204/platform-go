import os
import logging
import hmac
import ipaddress
import base64
from functools import wraps
from datetime import datetime, timezone
from contextlib import contextmanager

import psycopg2
from psycopg2 import pool, extras
from flask import Flask, request, jsonify, abort

# --- Configuration ---
class Config:
    LOG_LEVEL = os.environ.get('LOG_LEVEL', 'INFO').upper()
    DATABASE_URL = os.environ.get('DATABASE_URL')
    
    # Fallback to individual credentials if DATABASE_URL is unset
    if not DATABASE_URL:
        _USER = os.environ.get('PG_USER')
        _PASS = os.environ.get('PG_PASSWORD')
        _HOST = os.environ.get('PG_HOST', 'postgres')
        _PORT = os.environ.get('PG_PORT', '5432')
        _DB = os.environ.get('PG_DB', 'platform')
        if _USER and _PASS:
            DATABASE_URL = f'postgres://{_USER}:{_PASS}@{_HOST}:{_PORT}/{_DB}'
        else:
            pass

    WEBHOOK_SECRET = os.environ.get('WEBHOOK_SECRET')
    ALLOW_TRUSTED_IP_BYPASS = os.environ.get('ALLOW_TRUSTED_IP_BYPASS', 'false').lower() in ('true', '1', 'yes')
    TRUSTED_CIDRS = [ipaddress.ip_network(c.strip()) for c in os.environ.get('TRUSTED_IPS', '10.244.0.0/16').split(',') if c.strip()]
    PORT = int(os.environ.get('PORT', 8080))

# --- Logging Setup ---
logging.basicConfig(level=Config.LOG_LEVEL, format='%(asctime)s - %(levelname)s - %(name)s - %(message)s')
logger = logging.getLogger("webhook-service")

app = Flask(__name__)

# --- Database Pool ---
db_pool = None
if Config.DATABASE_URL:
    try:
        db_pool = psycopg2.pool.ThreadedConnectionPool(
            minconn=1, maxconn=20, dsn=Config.DATABASE_URL
        )
        logger.info("Database connection pool initialized.")
    except Exception as e:
        logger.critical(f"Failed to create DB pool: {e}")
else:
    logger.warning("No DATABASE_URL configured. Database features will be disabled.")

@contextmanager
def get_db_cursor(commit=True):
    """Yields a cursor and handles transaction commit/rollback/close automatically."""
    if not db_pool:
        raise RuntimeError("Database pool is not initialized.")
    
    conn = db_pool.getconn()
    try:
        with conn.cursor() as cur:
            yield cur
        if commit:
            conn.commit()
    except Exception as e:
        conn.rollback()
        logger.error(f"Database transaction error: {e}")
        raise
    finally:
        db_pool.putconn(conn)

# --- Authentication Helper ---
def is_trusted_ip(ip_addr):
    if not ip_addr: return False
    try:
        ip = ipaddress.ip_address(ip_addr)
        return any(ip in cidr for cidr in Config.TRUSTED_CIDRS)
    except ValueError:
        return False

def verify_signature(f):
    """Decorator to enforce Webhook Secret or Trusted IP validation."""
    @wraps(f)
    def decorated_function(*args, **kwargs):
        # 1. Check Secret (if configured)
        if Config.WEBHOOK_SECRET:
            token = (
                request.headers.get('X-Harbor-Token') or 
                request.headers.get('X-Webhook-Token') or 
                extract_auth_token(request.headers.get('Authorization'))
            )
            
            # Constant-time comparison
            if token and hmac.compare_digest(token, Config.WEBHOOK_SECRET):
                return f(*args, **kwargs)

        # 2. Check Trusted IP Bypass
        remote_ip = request.remote_addr or request.environ.get('REMOTE_ADDR')
        if Config.ALLOW_TRUSTED_IP_BYPASS and is_trusted_ip(remote_ip):
            logger.warning(f"Bypassed auth for trusted IP: {remote_ip}")
            return f(*args, **kwargs)

        logger.warning(f"Unauthorized access attempt from {remote_ip}")
        return jsonify({'error': 'Unauthorized'}), 401
    return decorated_function

def extract_auth_token(auth_header):
    """Parses Bearer, Basic, or plain tokens from Authorization header."""
    if not auth_header: return None
    parts = auth_header.split()
    if len(parts) == 2:
        scheme, value = parts[0].lower(), parts[1]
        if scheme == 'basic':
            try:
                # Decode user:pass. We treat the whole decoded string or the password as token
                decoded = base64.b64decode(value).decode('utf-8')
                return decoded.split(':')[1] if ':' in decoded else decoded
            except Exception:
                return None
        return value # Bearer or Token
    return auth_header # Fallback for bare token

# --- Database Logic ---

def upsert_repo_tag(cur, repo_name, tag_name):
    """Ensures Repo and Tag exist, returns Tag ID."""
    # 1. Get or Create Repo
    cur.execute("SELECT id FROM repos WHERE full_name=%s", (repo_name,))
    repo_row = cur.fetchone()
    if not repo_row:
        try:
            cur.execute("INSERT INTO repos (full_name) VALUES (%s) RETURNING id", (repo_name,))
            repo_row = cur.fetchone()
        except psycopg2.IntegrityError:
            cur.connection.rollback()
            cur.execute("SELECT id FROM repos WHERE full_name=%s", (repo_name,))
            repo_row = cur.fetchone()
    repo_id = repo_row[0]

    # 2. Get or Create Tag
    cur.execute("SELECT id FROM tags WHERE repository_id=%s AND tag=%s", (repo_id, tag_name))
    tag_row = cur.fetchone()
    if not tag_row:
        try:
            cur.execute("INSERT INTO tags (repository_id, tag) VALUES (%s, %s) RETURNING id", (repo_id, tag_name))
            tag_row = cur.fetchone()
        except psycopg2.IntegrityError:
            cur.connection.rollback() # Required to reset state in sub-transaction
            cur.execute("SELECT id FROM tags WHERE repository_id=%s AND tag=%s", (repo_id, tag_name))
            tag_row = cur.fetchone()
    
    return tag_row[0] if tag_row else None

def process_pull_event(repo, tag):
    now = datetime.now(timezone.utc)
    with get_db_cursor() as cur:
        tag_id = upsert_repo_tag(cur, repo, tag)
        if not tag_id:
            raise ValueError(f"Could not resolve tag_id for {repo}:{tag}")

        # Update Pull Status in a portable way (avoid relying on ON CONFLICT unique constraint)
        cur.execute('UPDATE image_pulls SET is_pulled=true, last_pulled_at=%s WHERE tag_id=%s', (now, tag_id))
        if cur.rowcount == 0:
            try:
                cur.execute('INSERT INTO image_pulls (tag_id, is_pulled, last_pulled_at) VALUES (%s, true, %s)', (tag_id, now))
            except psycopg2.IntegrityError:
                # Race condition: another transaction created the row. Rollback the failed statement and attempt update again.
                cur.connection.rollback()
                cur.execute('UPDATE image_pulls SET is_pulled=true, last_pulled_at=%s WHERE tag_id=%s', (now, tag_id))

        # Update Business Logic
        cur.execute("""
            UPDATE allowed_images 
            SET tag_id=%s, raw_name=%s, raw_tag=%s 
            WHERE name=%s AND tag=%s
        """, (tag_id, repo, tag, repo, tag))

        # If there was no existing allowed_images row, create one and mark it as global.
        if cur.rowcount == 0:
            try:
                cur.execute(
                    """
                    INSERT INTO allowed_images (name, tag, tag_id, raw_name, raw_tag, is_global, status, created_at, updated_at)
                    VALUES (%s, %s, %s, %s, %s, true, 'approved', %s, %s)
                    """,
                    (repo, tag, tag_id, repo, tag, now, now),
                )
            except psycopg2.IntegrityError:
                # Race condition: another transaction inserted the row. Rollback and update instead.
                cur.connection.rollback()
                cur.execute("""
                    UPDATE allowed_images 
                    SET tag_id=%s, raw_name=%s, raw_tag=%s 
                    WHERE name=%s AND tag=%s
                """, (tag_id, repo, tag, repo, tag))
    
    logger.info(f"Marked PULLED: {repo}:{tag}")

def process_delete_event(repo, tag=None):
    now = datetime.now(timezone.utc)
    with get_db_cursor() as cur:
        # Soft delete allowed_images
        query_allowed = "UPDATE allowed_images SET deleted_at=%s WHERE name=%s"
        params_allowed = [now, repo]
        
        if tag:
            query_allowed += " AND tag=%s"
            params_allowed.append(tag)
            
        cur.execute(query_allowed, tuple(params_allowed))

        # Reset is_pulled status
        if tag:
            cur.execute("""
                UPDATE image_pulls SET is_pulled=false 
                FROM tags t, repos r
                WHERE image_pulls.tag_id = t.id AND t.repository_id = r.id
                AND r.full_name = %s AND t.tag = %s
            """, (repo, tag))
        else:
            cur.execute("""
                UPDATE image_pulls SET is_pulled=false 
                FROM tags t, repos r
                WHERE image_pulls.tag_id = t.id AND t.repository_id = r.id
                AND r.full_name = %s
            """, (repo,))

    logger.info(f"Marked DELETED: {repo}:{tag if tag else '*'}")

# --- Payload Parser (FIXED) ---

def extract_harbor_resources(payload):
    """
    Unified parser for Harbor v1/v2 webhooks.
    Handles nested 'event_data' structure correctly.
    Returns a list of tuples: [(repo_name, tag_name), ...]
    """
    resources = []
    
    # --- Strategy 1: 'event_data' (Harbor 2.x standard) ---
    # 大部分 Harbor 事件都在 event_data 裡
    data_node = payload.get('event_data') or payload.get('artifact') or payload
    if not isinstance(data_node, dict):
        return []

    # [關鍵修正]: 先嘗試在最上層尋找 Repository 資訊
    # 因為在 push_artifact 事件中，Repository 資訊通常與 resources 是兄弟節點(Sibling)，而不是在 resources 裡面
    global_repo_info = data_node.get('repository', {})
    global_repo_name = global_repo_info.get('repo_full_name') or global_repo_info.get('name')

    # 取得資源列表
    res_list = data_node.get('resources', [])
    
    # 兼容舊版或特殊格式：如果是單一物件 resource
    if not res_list and data_node.get('resource'):
         res_list = [data_node.get('resource')]

    # 遍歷資源列表
    for res in res_list:
        # 1. 嘗試從單一資源中取得 Repo 名稱
        repo_obj = res.get('repository') or res.get('repo') or res.get('resource')
        current_repo = None
        
        if isinstance(repo_obj, dict):
            current_repo = repo_obj.get('repo_full_name') or repo_obj.get('name')
        elif isinstance(repo_obj, str):
            current_repo = repo_obj
        
        # 2. 如果單一資源沒有 Repo 名稱，使用全域的 (Fallback to global)
        if not current_repo:
            current_repo = global_repo_name
            
        # 3. 處理 Tags
        tags = res.get('tags', [])
        if res.get('tag'): 
            tags.append(res.get('tag'))
        
        if current_repo:
            if not tags: 
                # Case: Delete event on Repo level (no specific tag)
                resources.append((current_repo, None))
            else:
                for t in tags:
                    t_name = t.get('name') if isinstance(t, dict) else t
                    resources.append((current_repo, t_name))
    
    if resources: return resources

    # --- Strategy 2: Legacy/Fallback (push_data) ---
    # 處理舊版 Harbor 格式
    repo = payload.get('repository', {}).get('name')
    tag = (payload.get('push_data') or {}).get('tag')
    
    if repo and tag:
        resources.append((repo, tag))
    elif repo and 'delete' in (payload.get('type', '').lower()):
        resources.append((repo, None))

    return resources

# --- Routes ---

@app.route('/webhook', methods=['POST'])
@verify_signature
def webhook():
    try:
        payload = request.get_json(force=True)
    except Exception:
        return jsonify({'error': 'Invalid JSON'}), 400

    if not payload:
        return jsonify({'status': 'empty'}), 400

    # Determine Action
    event_type = (payload.get('type') or payload.get('action') or '').lower()
    is_delete = 'delete' in event_type

    # Parse Data (Using the FIXED function)
    items = extract_harbor_resources(payload)
    
    if not items:
        # Debugging: Log payload keys if parsing fails, similar to the fix suggestion
        data_keys = list(payload.get('event_data', {}).keys()) if payload.get('event_data') else list(payload.keys())
        logger.warning(f"Ignored event: {event_type} (no resources found). Keys available: {data_keys}")
        return jsonify({'status': 'ignored'}), 200

    success_count = 0
    for repo, tag in items:
        try:
            if is_delete:
                process_delete_event(repo, tag)
            else:
                # Treat as Push/Pull
                if tag: process_pull_event(repo, tag)
            success_count += 1
        except Exception:
            logger.exception(f"Error processing {repo}:{tag}")
            # Continue processing other items, but log error
    
    return jsonify({'status': 'ok', 'processed': success_count}), 200

@app.route('/healthz', methods=['GET'])
def healthz():
    # Simple DB check
    try:
        with get_db_cursor() as cur:
            cur.execute("SELECT 1")
    except Exception:
        return jsonify({'status': 'db_error'}), 500
    return jsonify({'status': 'ok'}), 200

if __name__ == '__main__':
    # Not for production, use Gunicorn
    app.run(host='0.0.0.0', port=Config.PORT)
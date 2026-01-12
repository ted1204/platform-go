Harbor ↔ Postgres sync examples

Overview
- `webhook_receiver.py`: a small Flask app that accepts Harbor webhook push events and marks matching rows in Postgres `images(repository, tag)` with `is_pulled = true`.
- `reconcile_harbor.py`: a one-shot script that pulls artifact lists from Harbor and updates `images.is_pulled` to reflect existence in Harbor (true/false).

Important environment variables
- `DATABASE_URL` — Postgres DSN (postgres://user:pass@host:port/dbname)
- `HARBOR_API` — Harbor API base, e.g. https://harbor.example.com/api/v2.0
- `HARBOR_USER` and `HARBOR_PASS` — credentials for Harbor API
- `HARBOR_PROJECT` — optional: limit reconcile to one project

Usage
1) Install deps:
```
pip install -r scripts/harbor_sync/requirements.txt
```

2) Run webhook receiver (example):
```
export DATABASE_URL='postgres://user:pass@db:5432/mydb'
python scripts/harbor_sync/webhook_receiver.py
```

Configure Harbor project webhook to POST to `http://<host>:8080/webhook`.

3) Run manual reconcile:
```
export HARBOR_API=https://harbor.example.com/api/v2.0
export HARBOR_USER=admin
export HARBOR_PASS=secret
export DATABASE_URL='postgres://user:pass@db:5432/mydb'
python scripts/harbor_sync/reconcile_harbor.py
```

Notes & Next steps
- Adjust SQL and table/column names to match your schema. The examples assume a table `images(repository,text, tag,text, is_pulled boolean)` with a unique constraint on (repository, tag).
- For production: run Flask app behind a WSGI server (gunicorn) and secure the endpoint (token verification, TLS).
- Consider deploying `reconcile_harbor.py` as a K8s CronJob to regularly fix drift.

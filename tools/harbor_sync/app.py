from webhook_receiver import app

# gunicorn expects a module-level `app` variable; this file is a thin wrapper
# so the container can run: `gunicorn -w 4 --threads 2 -b 0.0.0.0:8080 app:app`

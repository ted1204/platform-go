FROM postgres:latest

RUN apt-get update && \
    apt-get install -y postgresql-17-cron && \
    rm -rf /var/lib/apt/lists/*

RUN echo "shared_preload_libraries = 'pg_cron'" >> /usr/share/postgresql/postgresql.conf.sample
RUN echo "cron.database_name = 'platform'" >> /usr/share/postgresql/postgresql.conf.sample
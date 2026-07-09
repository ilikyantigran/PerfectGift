#!/bin/sh
# Runs once, on first Postgres init (empty data dir), as the postgres superuser.
#
# Creates one database per backend service (own-schema isolation — no cross-service
# DB access). identity/poll/surprise self-migrate at boot via embedded migrations,
# so they only need their database to exist. catalog and notification do NOT
# self-migrate, so we apply their schema here from the SQL mounted under /seed.
set -e

echo "[init] creating per-service databases"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-'EOSQL'
	CREATE DATABASE perfectgift;   -- identity
	CREATE DATABASE poll;
	CREATE DATABASE surprise;
	CREATE DATABASE catalog;
	CREATE DATABASE notification;
EOSQL

echo "[init] applying catalog schema (no self-migrate)"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname catalog -f /seed/catalog.sql

echo "[init] applying notification schema (no self-migrate)"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname notification -f /seed/notification.sql

echo "[init] done"

#!/bin/bash
# =========================================================
# PostgreSQL Multiple Database Initialization Script
# =========================================================
#
# DESCRIPTION:
#   This script automatically creates multiple PostgreSQL databases
#   during container initialization. It utilizes the container's
#   /docker-entrypoint-initdb.d/ initialization hook.
#
# USAGE:
#   1. Mount this script to /docker-entrypoint-initdb.d/ in the PostgreSQL container
#   2. Set POSTGRES_MULTIPLE_DATABASES env var with comma-separated database names
#
# ENVIRONMENT VARIABLES:
#   - POSTGRES_MULTIPLE_DATABASES: Comma-separated list of databases to create
#   - POSTGRES_USER: Database user (will be granted access to all created DBs)
#

set -e

# Use env var or default list of databases
if [ -z ${POSTGRES_MULTIPLE_DATABASES+x} ]; then
    POSTGRES_MULTIPLE_DATABASES="canister,device,sales,sales_readonly,truck"
fi

function create_user_and_database() {
    local database=$1
    echo "  Creating user and database '$database'"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
        CREATE DATABASE $database;
        GRANT ALL PRIVILEGES ON DATABASE $database TO $POSTGRES_USER;
EOSQL
}

# Check if POSTGRES_USER is set, default to postgres
if [ -z ${POSTGRES_USER+x} ]; then
    POSTGRES_USER=postgres
fi

echo "Multiple database creation requested: $POSTGRES_MULTIPLE_DATABASES"
for db in $(echo $POSTGRES_MULTIPLE_DATABASES | tr ',' ' '); do
    create_user_and_database $db
done
echo "Multiple databases created"
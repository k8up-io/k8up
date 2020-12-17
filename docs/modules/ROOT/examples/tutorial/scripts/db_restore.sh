#!/usr/bin/env sh

# This script restores the contents of a backup to the MariaDB database
# It is copied into a running pod and executed inside it
# by the `4_restore.sh` script.

echo "Restoring database in pod"
mysql -uroot -p"${MARIADB_ROOT_PASSWORD}" < /backup.sql

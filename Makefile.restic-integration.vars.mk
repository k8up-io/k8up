arch ?= amd64

ifeq ("$(shell uname -s)", "Darwin")
os ?= darwin
else
os ?= linux
endif

curl_args ?= --location --fail --silent --show-error

backup_dir ?= $(integrationtest_dir)/backup
restore_dir ?= $(integrationtest_dir)/restore

stats_url ?= http://localhost:8091

restic_version ?= $(shell go mod edit -json | jq -r '.Require[] | select(.Path == "github.com/restic/restic").Version' | sed "s/v//")
restic_path ?= $(go_bin)/restic
restic_pid ?= $(integrationtest_dir)/restic.pid
restic_url ?= https://github.com/restic/restic/releases/download/v$(restic_version)/restic_$(restic_version)_$(os)_$(arch).bz2
restic_password ?= repopw

minio_port ?= 9000
minio_host ?= localhost
minio_address = $(minio_host):$(minio_port)
minio_path ?= $(go_bin)/minio
minio_data ?= $(integrationtest_dir)/minio.d/data
minio_config ?= $(integrationtest_dir)/minio.d/config
minio_root_user ?= accesskey
minio_root_password ?= secretkey
minio_pid ?= $(integrationtest_dir)/minio.pid
minio_url ?= https://dl.min.io/server/minio/release/$(os)-$(arch)/minio

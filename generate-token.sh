#!/bin/bash

FORCE=0
if [ "$1" = "--force" ]; then
    FORCE=1
fi

if [ -f .env ]; then
    source .env
fi

if [ -n "$INFLUXDB_TOKEN" ] && [ $FORCE -eq 0 ]; then
    echo "influxdb token already set"
    exit 0
fi

if ! command -v openssl &> /dev/null; then
    echo "Error: openssl command not found.. Exiting!"
    exit 1
fi


TOKEN=$(openssl rand -base64 64 | tr -d '\n')

echo INFLUXDB_TOKEN="$TOKEN" > .env
echo "Generated .env with token"

if ! command -v envsubst &> /dev/null; then
    echo "Error: envsubst command not found.. Exiting!"
fi

source .env
export INFLUXDB_TOKEN
envsubst < config/config.template.yaml > config/config.yaml
echo "Generated config/config.yaml"

envsubst < provisioning/datasources/influxdb.template.yml > provisioning/datasources/influxdb.yml
echo "Generated provisioning/datasources/influxdb.yml"

if [ $FORCE -eq 1 ]; then
    echo "Token regenerated. Now run 'make reset' to remove volumes and reinitialize InfluxDB, then 'make run'."
fi

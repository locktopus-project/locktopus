set -eux

docker-compose -f docker/e2e/docker-compose.yml build

docker-compose -f docker/e2e/docker-compose.yml up -d server

for var in v1; do
    docker-compose -f docker/e2e/docker-compose.yml up --exit-code-from v1 v1
done

docker-compose -f docker/e2e/docker-compose.yml down

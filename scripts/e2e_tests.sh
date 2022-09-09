set -eux

docker-compose -f docker/e2e/docker-compose.yml build

docker-compose -f docker/e2e/docker-compose.yml up -d server

for var in v1; do
    if docker-compose -f docker/e2e/docker-compose.yml up --exit-code-from $var $var; then
        echo test $var passed
    else
        exit 1
    fi

done

docker-compose -f docker/e2e/docker-compose.yml down

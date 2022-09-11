set -eux

docker-compose -f docker/e2e/docker-compose.yml build

docker-compose -f docker/e2e/docker-compose.yml up -d server

CODE=0

for var in stats_v1; do
    if docker-compose -f docker/e2e/docker-compose.yml up --exit-code-from $var $var; then
        echo test $var passed
    else
        CODE=1
        break
    fi

done

docker-compose -f docker/e2e/docker-compose.yml down

exit $CODE

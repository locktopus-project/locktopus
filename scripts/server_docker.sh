# Builds and runs server with default settings inside a container
docker build -t locktopus . && docker run -it --rm --net=locktopus --name server locktopus

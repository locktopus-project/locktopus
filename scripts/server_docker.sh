# Builds and runs server with default settings inside a container
docker build -t gearlock . && docker run -it --rm --net=gearlock --name server gearlock

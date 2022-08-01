mkdir -p pprof
go build -o ./bin/simulation cmd/simulation/simulation.go && ./bin/simulation -cpuprofile "pprof/prof"
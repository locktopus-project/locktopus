mkdir -p pprof
go build -o ./bin/simulation cmd/simulation/simulation.go && chmod +x ./bin/simulation && ./bin/simulation -cpuprofile "pprof/prof"
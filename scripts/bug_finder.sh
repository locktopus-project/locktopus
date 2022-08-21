mkdir -p pprof
go build -o ./bin/bug_finder ./cmd/bug_finder && chmod +x ./bin/bug_finder && ./bin/bug_finder 
# go build -o ./bin/bug_finder ./cmd/bug_finder && chmod +x ./bin/bug_finder && ./bin/bug_finder -cpuprofile "pprof/prof"
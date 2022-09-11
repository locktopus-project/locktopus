# Runs infinite process of finding a bug in the locking logic. If panic occured, a bug is found.
go build -o ./bin/bug_finder ./cmd/bug_finder && chmod +x ./bin/bug_finder && ./bin/bug_finder

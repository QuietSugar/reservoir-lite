.DEFAULT_GOAL := build

# Build the proxy executable
build:
	go build -o reservoir main.go

# Run the proxy
run: build
	./reservoir

# Remove build artifacts
clean:
	rm -f reservoir reservoir.exe
	rm -rf var/

.PHONY: build run clean

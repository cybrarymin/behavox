## help: print the help message
-include .envrc # -include will include .envrc but if it doesn't exist it won't return error. .envrc usually is not commited in git so to avoid pipeline failure we do this

#================================================================#
# HELPERS
#================================================================#

# always use helo as the first target. Because make command without any target will run first target defined in it. "make" will equal to "make help"
.PHONY: help # .PHONY for each target will teach make if we have a local file or directory that names help pls don't consider them and use the target we defined cause make command can't dinstingush the directory or file from targets we define inside makefile and it get's confused
help: # @ before the command will not echo the command itself when we run make <target> command
	@echo "Usage:" 
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

# installing the prerequisites required by other make targets such as staticcheck command.
# after installing required binaries we will add the GO binary path to the shell $PATH
.PHONY: prerequsite
prerequsite:
	@echo "Installing Go required tools..."
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "Updating shell configuration..."
	@GOPATH=$$(go env GOPATH); \
	PATH_EXPORT="export PATH=\$$PATH:\$$GOPATH/bin"; \
	for RC in $$HOME/.zshrc $$HOME/.bashrc; do \
		if [ -f "$$RC" ] && ! grep -q "export PATH=.*GOPATH" "$$RC"; then \
			echo "\n# Go binaries path\n$$PATH_EXPORT" >> "$$RC"; \
		fi; \
	done; 
	@echo "Prerequisites installed successfully!"



#================================================================#
# DEVELOPMENT
#================================================================#
## build/api: building the application
current_time = $(shell date +"%Y-%m-%dT%H:%M:%S%z")
git_version = $(shell git describe --always --long --dirty --tags 2>/dev/null; if [[ $$? != 0 ]]; then git describe --always --dirty; fi)

Linkerflags = -s -X github.com/cybrarymin/behavox/api.BuildTime=${current_time} -X github.com/cybrarymin/behavox/api.Version=${git_version}
.PHONY: build/api
build/api:
	@go mod tidy
	GOOS=linux GOARCH=amd64 go build -ldflags="${Linkerflags}" -o=./bin/behavox-linux-amd64 ./
	GOOS=darwin GOARCH=arm64 go build -ldflags="${Linkerflags}" -o=./bin/behavox-darwin-arm64 ./
	go build -o=./bin/behavox-local-compatible -ldflags="${Linkerflags}" ./

## run/api: runs the application on port 443 with custom self signed certificate
.PHONY: run/api
run/api:
	@if [ -f /tmp/cert.pem ] && [ -f /tmp/key.pem ]; then \
		openssl req -x509 -newkey rsa:4096 -keyout /tmp/key.pem -out /tmp/cert.pem -sha256 -days 3650 -nodes -subj "/C=CA/ST=ON/L=Toronto/O=Behavox/OU=Devops/CN=*.behavox.com" 1>&2 2>/dev/null; \
		fi;
	@go run -race main.go  \
	--log-level=${LOGLVL} \
	--listen-addr ${LISTEN_ADDR} \
	--cert /tmp/cert.pem  \
	--cert-key /tmp/key.pem \
	--enable-rate-limit=false \
	--event-queue-size=100 \
	--event-queue-max-worker-threads=10 \
	--api-admin-user ${ADMINUSER} \
	--api-admin-pass ${ADMINPASS} \
	--jwkey ${JWTKEY}


#================================================================#
# QUALITY CHECK , LINTING, Vendoring
#================================================================#
## audit: runs the application audit checks such as tests, lintings, staticchecks and ....
.PHONY: audit
audit: prerequsite
	@echo "Tidying and verifying golang packages and module dependencies..."
	go mod tidy
	go mod verify
	@echo "Formatting codes..."
	go fmt ./...
	@echo "Vetting codes..."
	go vet ./...
	@echo "Static Checking of the code..."
	staticcheck ./...
	@echo "Running tests..."
	go test -race -vet=off ./...

.PHONY: vendor
vendor:
	@echo "Tidying and verifying golang packages and module dependencies..."
	go mod verify
	go mod tidy
	@echo "Vendoring all golang dependency modules and packages..."
	go mod vendor


#================================================================#
# Swagger documentation
#================================================================#
## docs/swagger: is used to generate the OpenAPIswagger docs 
.PHONY: docs/swagger
docs/swagger:
	@swag fmt
	@swag init -g cmd/api/swagger.go
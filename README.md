# sysl-go-comms

Communication library used by SYSL-generated code written in Go.

## 1.1. Getting Started

Go get the repository

    go get github.com/anz-bank/sysl-go-comms

### 1.1.1. Local Development

### 1.1.2. Prerequisites

Ensure your environment provides:

- [go 1.12.9](https://golang.org/)
- [golangci-lint 1.17.1](https://github.com/golangci/golangci-lint)
- some working method of obtaining dependencies listed in `go.mod` (working internet access, `GOPROXY`)
- env var `GOFLAGS="-mod=vendor"`
- correctly configured cntlm or alpaca proxy running on localhost at port 3128 (only for updating vendor dependencies)

### 1.1.3. Linting
    golangci-lint run ./...

### 1.1.4. Running the Tests
    go test -v -cover ./...

To generate and view test coverage in a browser, use this

    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out


## 1.2. CI/CD

The anz-bank/sysl-go-comms repository has been configured to automatically run lint checks and execute unit tests on commit. For more details, refer to [GCP Cloudbuild CI](docs/README-GCP-CLOUDBUILD-CI.md)

## 1.3. Updating Vendor Dependencies

    go mod tidy
    go mod vendor

carefully review the changes and check them in if they are correct.

Beware: `go mod vendor` does not correctly vendor some dependencies that contain non-go assets. See [go/issues/26366](https://github.com/golang/go/issues/26366). A third party workaround is offered here: [nomad-software/vend](https://github.com/nomad-software/vend).

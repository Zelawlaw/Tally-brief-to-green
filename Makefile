.PHONY: build unit functional lint sonar gates clean

APP_BIN := tally
COVERAGE_OUT := coverage.out
SONAR_HOST ?= http://localhost:9000
SONAR_PROJECT ?= tally
SONAR_USER ?= admin
SONAR_PASS ?= admin
TALLY_URL ?= http://localhost:8080

build:
	go vet ./...
	go build -o $(APP_BIN) ./cmd/tally

unit:
	go test ./... -count=1 -coverprofile=$(COVERAGE_OUT) -covermode=atomic

functional:
	@curl -s -o /dev/null $(TALLY_URL) || docker compose up -d app --build
	TALLY_BASE_URL=$(TALLY_URL) go test ./... -tags=functional -count=1

lint:
	golangci-lint run ./...

sonar: unit
	@curl -s -o /dev/null $(SONAR_HOST)/api/system/status || (docker compose up -d sonarqube && ./sonar/wait-for-sonar.sh $(SONAR_HOST) && ./sonar/bootstrap.sh $(SONAR_HOST) $(SONAR_PROJECT))
	sonar-scanner -Dsonar.host.url=$(SONAR_HOST) -Dsonar.projectKey=$(SONAR_PROJECT) -Dsonar.login=$(SONAR_USER) -Dsonar.password=$(SONAR_PASS)
	@echo "Polling SonarQube quality gate for project '$(SONAR_PROJECT)'..."
	@./sonar/check-gate.sh $(SONAR_HOST) $(SONAR_PROJECT)

gates: build unit functional lint sonar
	@echo "=== All gates passed ==="

dev-down:
	docker compose down

clean:
	rm -f $(APP_BIN) $(COVERAGE_OUT)

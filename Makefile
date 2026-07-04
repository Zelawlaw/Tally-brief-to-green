.PHONY: build unit functional lint sonar gates clean dev-up dev-down

APP_BIN := tally
COVERAGE_OUT := coverage.out
SONAR_HOST := http://localhost:9001
SONAR_PROJECT := tally
SONAR_TOKEN ?=

build:
	go vet ./...
	go build -o $(APP_BIN) ./cmd/tally

unit:
	go test ./... -count=1 -coverprofile=$(COVERAGE_OUT) -covermode=atomic

functional:
	go test ./... -tags=functional -count=1

lint:
	golangci-lint run ./...

sonar:
	sonar-scanner -Dsonar.host.url=$(SONAR_HOST) -Dsonar.projectKey=$(SONAR_PROJECT) -Dsonar.token=$(SONAR_TOKEN)
	@echo "Polling SonarQube quality gate for project '$(SONAR_PROJECT)'..."
	@./sonar/check-gate.sh $(SONAR_HOST) $(SONAR_PROJECT)

gates: build unit functional lint sonar
	@echo "=== All gates passed ==="

dev-up:
	docker compose up -d sonarqube
	@echo "Waiting for SonarQube to be ready at $(SONAR_HOST)..."
	@./sonar/wait-for-sonar.sh $(SONAR_HOST)
	@./sonar/bootstrap.sh $(SONAR_HOST) $(SONAR_PROJECT)

dev-down:
	docker compose down

clean:
	rm -f $(APP_BIN) $(COVERAGE_OUT)

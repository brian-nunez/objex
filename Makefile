tests:
	@go test -v -cover -race ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out

coverage: tests
	@go tool cover -html=coverage.out -o coverage.html
	@open coverage.html

clean:
	@rm -f coverage.out coverage.html

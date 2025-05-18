# Makefile

APP_NAME := balancerApp
MAIN_FILE := ./cmd/app/main.go

.PHONY: run build clean

run: build
	./$(APP_NAME)

build:
	go build -o $(APP_NAME) $(MAIN_FILE)

clean:
	rm -f $(APP_NAME)

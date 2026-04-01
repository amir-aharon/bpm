.PHONY: build run

build:
	docker build -t bpm .

run:
	docker run -p 8080:8080 bpm

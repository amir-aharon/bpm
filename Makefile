.PHONY: build run

build:
	go build -o bpm .

run:
	./bpm -serve

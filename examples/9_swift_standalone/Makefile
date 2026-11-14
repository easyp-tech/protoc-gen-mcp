.PHONY: generate lint build run clean

generate:
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml generate -p proto -r .
	rm -rf google

lint:
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml lint -p proto -r .

build:
	swift build

run: build
	swift run NotebookServer

clean:
	rm -rf .build google

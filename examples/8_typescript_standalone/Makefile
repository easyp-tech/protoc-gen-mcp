.PHONY: setup generate lint build run clean

setup:
	npm ci

generate: setup
	npm run generate
	go run ../../internal/devtools/protobufesnormalize $$(find src/generated -name '*_pb.ts' -type f | sort)

lint:
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml lint -p proto -r .

build: generate
	npm run typecheck
	npm run build

run: build
	npm run run

clean:
	rm -rf node_modules dist google src/generated

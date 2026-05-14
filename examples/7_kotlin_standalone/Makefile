.PHONY: build clean generate lint run

generate:
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml generate -p proto -r .
	rm -rf google src/generated/main/java/com/google

lint:
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml lint -p proto -r .

build:
	gradle --no-daemon build

run:
	gradle --no-daemon run

clean:
	rm -rf .gradle build src/generated google

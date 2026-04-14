.PHONY: generate lint

generate:
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml generate -p . -r .
	rm -rf google
	$(MAKE) -C 5_python_standalone generate

lint:
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml lint -p . -r .
	$(MAKE) -C 5_python_standalone lint

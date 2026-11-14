.PHONY: clean-build-artifacts generate lint

clean-build-artifacts:
	rm -rf 6_java_standalone/build 7_kotlin_standalone/build
	rm -rf 8_typescript_standalone/dist
	rm -rf 9_swift_standalone/.build

generate:
	$(MAKE) clean-build-artifacts
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml generate -p . -r .
	$(MAKE) -C 5_python_standalone generate
	$(MAKE) -C 6_java_standalone generate
	$(MAKE) -C 7_kotlin_standalone generate
	$(MAKE) -C 8_typescript_standalone generate
	$(MAKE) -C 9_swift_standalone generate

lint:
	$(MAKE) clean-build-artifacts
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml lint -p . -r .
	$(MAKE) -C 5_python_standalone lint
	$(MAKE) -C 6_java_standalone lint
	$(MAKE) -C 7_kotlin_standalone lint
	$(MAKE) -C 8_typescript_standalone lint
	$(MAKE) -C 9_swift_standalone lint

.PHONY: clean-build-artifacts generate lint

clean-build-artifacts:
	rm -rf jvm/*/build 6_java_standalone/build 7_kotlin_standalone/build
	rm -rf 8_typescript_standalone/dist 9_javascript_standalone/node_modules

generate:
	$(MAKE) clean-build-artifacts
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml generate -p . -r .
	rm -rf google
	$(MAKE) -C 5_python_standalone generate
	$(MAKE) -C 10_python_protobuf_standalone generate
	$(MAKE) -C 6_java_standalone generate
	$(MAKE) -C 7_kotlin_standalone generate
	$(MAKE) -C 8_typescript_standalone generate

lint:
	$(MAKE) clean-build-artifacts
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml lint -p . -r .
	$(MAKE) -C 5_python_standalone lint
	$(MAKE) -C 10_python_protobuf_standalone lint
	$(MAKE) -C 6_java_standalone lint
	$(MAKE) -C 7_kotlin_standalone lint
	$(MAKE) -C 8_typescript_standalone lint

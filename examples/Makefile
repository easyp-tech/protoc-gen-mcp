.PHONY: clean-jvm-build-artifacts generate lint

clean-jvm-build-artifacts:
	rm -rf jvm/*/build 6_java_standalone/build 7_kotlin_standalone/build

generate:
	$(MAKE) clean-jvm-build-artifacts
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml generate -p . -r .
	rm -rf google
	$(MAKE) -C 5_python_standalone generate
	$(MAKE) -C 6_java_standalone generate
	$(MAKE) -C 7_kotlin_standalone generate

lint:
	$(MAKE) clean-jvm-build-artifacts
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml lint -p . -r .
	$(MAKE) -C 5_python_standalone lint
	$(MAKE) -C 6_java_standalone lint
	$(MAKE) -C 7_kotlin_standalone lint

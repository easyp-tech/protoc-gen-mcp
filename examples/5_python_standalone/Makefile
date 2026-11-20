.PHONY: setup generate lint run clean

PYTHON ?= python3
VENV ?= .venv
VENV_PYTHON := $(VENV)/bin/python
VENV_STAMP := $(VENV)/.installed
# protoc-gen-mcp-runtime is not on PyPI yet: install it from this repository.
# Outside the repository, point RUNTIME at the wheel attached to the release.
RUNTIME ?= ../../runtime/python

$(VENV_PYTHON):
	$(PYTHON) -m venv $(VENV)

$(VENV_STAMP): pyproject.toml $(wildcard $(RUNTIME)/pyproject.toml) | $(VENV_PYTHON)
	$(VENV_PYTHON) -m pip install --upgrade pip
	$(VENV_PYTHON) -m pip install $(RUNTIME) -e .
	touch $(VENV_STAMP)

setup: $(VENV_STAMP)

generate:
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml generate -p . -r .
	rm -rf google

lint:
	easyp --cfg easyp.yaml mod download
	easyp --cfg easyp.yaml lint -p . -r .

run: setup
	$(VENV_PYTHON) server.py

clean:
	rm -rf .venv google

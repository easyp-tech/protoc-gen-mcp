.PHONY: setup generate lint run clean

PYTHON ?= python3
VENV ?= .venv
VENV_PYTHON := $(VENV)/bin/python
VENV_STAMP := $(VENV)/.installed

$(VENV_PYTHON):
	$(PYTHON) -m venv $(VENV)

$(VENV_STAMP): pyproject.toml | $(VENV_PYTHON)
	$(VENV_PYTHON) -m pip install --upgrade pip
	$(VENV_PYTHON) -m pip install -e .
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

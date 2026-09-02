.PHONY: install run seed test test-front migrate front front-install

VENV ?= .venv

install:
	python3 -m venv $(VENV)
	$(VENV)/bin/pip install -r backend/requirements.txt

migrate:
	$(VENV)/bin/alembic upgrade head

seed:
	$(VENV)/bin/python -m backend.seed_data

run:
	$(VENV)/bin/uvicorn backend.main:app --reload --port 8000

test:
	$(VENV)/bin/python -m pytest

test-front:
	cd frontend && npm test

front-install:
	cd frontend && npm install

front:
	cd frontend && npm run dev

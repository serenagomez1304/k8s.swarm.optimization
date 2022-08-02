# syntax=docker/dockerfile:1

FROM python:3.8-slim-buster

WORKDIR /app

COPY employee.py ./
COPY logr.py ./
COPY sample_app.py ./

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir kubernetes

CMD ["python", "./employee.py"]

# syntax=docker/dockerfile:1

FROM python:3.8-slim-buster

WORKDIR /app

COPY bee.py ./
COPY logr.py ./
COPY linear.py ./

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir kubernetes

CMD ["python", "./linear.py"]

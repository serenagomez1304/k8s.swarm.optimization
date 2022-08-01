# syntax=docker/dockerfile:1

FROM python:3.8-slim-buster

WORKDIR /app

COPY onlooker-app.py ./

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir kubernetes

CMD ["python", "./onlooker-app.py"]

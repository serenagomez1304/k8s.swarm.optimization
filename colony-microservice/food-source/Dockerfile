# syntax=docker/dockerfile:1

FROM python:3.8-slim-buster

WORKDIR /app

COPY foodsource-app.py ./

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir kubernetes

CMD ["python", "./foodsource-app.py"]

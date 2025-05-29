FROM python:3.11-slim
RUN apt-get update -y && apt-get install -y gcc && apt-get clean && rm -rf /var/lib/apt/lists/*
LABEL maintainer="Neel"

WORKDIR /app

COPY requirements.txt ./
RUN pip install -r requirements.txt

COPY . .
CMD ["bash","start.sh"]
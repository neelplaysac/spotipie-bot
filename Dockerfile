FROM python:3.8.2-slim-buster
RUN apt-get update -y && apt-get install -y gcc
LABEL maintainer="Neel"

WORKDIR /usr/local/bin

COPY requirements.txt ./
RUN pip install -r requirements.txt

COPY . .
CMD ["bash","start.sh"]
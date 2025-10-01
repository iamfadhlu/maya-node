FROM python:3.10-slim
WORKDIR /app
ENV PYTHONPATH=/app

RUN apt-get update && apt-get install -y --no-install-recommends \
    libsecp256k1-2 libsecp256k1-dev \
    build-essential git pkg-config \
 && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install python-bitcointx
RUN pip install -r requirements.txt

COPY . .

RUN pip install python-bitcointx
RUN pip install -e /app/python-dashtx
RUN pip install -r requirements.txt

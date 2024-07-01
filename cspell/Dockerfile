FROM node:lts-alpine3.20

ENV PATH="/home/node/node_modules/.bin:$PATH"

WORKDIR /home/node

COPY package.json .
COPY package-lock.json .

RUN npm ci

WORKDIR /victoriametrics

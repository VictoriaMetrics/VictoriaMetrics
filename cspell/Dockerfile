FROM node:lts-alpine3.20

WORKDIR /opt/node
COPY yarn.lock package.json .
RUN yarn install
WORKDIR /vm
ENTRYPOINT ["/opt/node/node_modules/.bin/cspell"]

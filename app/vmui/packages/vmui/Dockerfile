FROM node:14-alpine3.12 as build-stage

RUN apk update && apk add --no-cache bash bash-doc bash-completion libtool autoconf automake nasm pkgconfig libpng gcc make g++ zlib-dev gawk

RUN mkdir -p /app
WORKDIR /app
COPY ./package.json /app/package.json
COPY ./package-lock.json /app/package-lock.json
RUN cd /app && npm install
COPY . /app
RUN npm run build

FROM nginx:latest as production-stage

COPY --from=build-stage /app/build /usr/share/nginx/html
COPY ./nginx/nginx.conf /etc/nginx/nginx.conf
COPY ./nginx/default /etc/nginx/sites-enabled/default
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
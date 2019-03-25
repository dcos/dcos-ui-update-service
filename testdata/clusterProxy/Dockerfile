FROM node:10-alpine

EXPOSE 7070

ADD package.json /tmp/package.json
RUN cd /tmp && npm install --quiet
RUN mkdir -p /opt/app && cp -a /tmp/node_modules /opt/app/

WORKDIR /opt/app
COPY . .
FROM node:10

WORKDIR /usr/src/app

COPY index.js ./
COPY package.json ./

RUN npm install

EXPOSE 7070

CMD ["npm", "start"]
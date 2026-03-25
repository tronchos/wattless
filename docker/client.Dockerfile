FROM node:20-bookworm

WORKDIR /app

COPY client/package.json client/package-lock.json* ./
RUN npm install

COPY client ./

ENV PORT=3000

CMD ["npm", "run", "dev", "--", "--hostname", "0.0.0.0"]


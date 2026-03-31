FROM node:20-bookworm

WORKDIR /app

COPY client/package.json client/package-lock.json* ./
RUN npm install

COPY client ./

ENV PORT=5173

CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]

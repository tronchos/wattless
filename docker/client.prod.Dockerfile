FROM node:20-bookworm AS deps

WORKDIR /app

COPY client/package.json client/package-lock.json ./
RUN npm ci

FROM node:20-bookworm AS builder

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY client ./

RUN npm run build

FROM node:20-bookworm-slim AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

RUN groupadd --gid 1001 nextjs \
  && useradd --uid 1001 --gid 1001 --create-home --home-dir /home/nextjs --shell /usr/sbin/nologin nextjs

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
RUN chown -R nextjs:nextjs /app

USER nextjs

EXPOSE 3000

CMD ["node", "server.js"]

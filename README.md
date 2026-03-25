# Wattless

Wattless es una demo de hackatón centrada en sostenibilidad web, rendimiento e IA. Combina un escáner en Go con `rod`, un BFF en Next.js y un dashboard pensado para explicar en segundos la relación entre `bytes + CO2 + LCP`.

## Qué hace

- Escanea una URL real y calcula `score`, transferencia y `CO2` por visita.
- Mide `LCP`, `FCP`, `load` y actividad de scripts.
- Revisa hosting verde con The Green Web Foundation.
- Destaca los 5 elementos vampiro sobre una captura interactiva.
- Genera `insights IA` con fallback a reglas.
- Produce un `Green Fix` demoable para un snippet de React/Next.js.
- Exporta un `Markdown report` listo para README o PR.
- Incluye showcase controlado en `/showcase/heavy` y `/showcase/wattless`.

## Estructura

- `client/`: frontend Next.js App Router y BFF same-origin.
- `server/`: API Go, scanner, cálculo de CO2 e integración de IA.
- `docker/`: contenedores de desarrollo y producción.
- `PROJECT.md`: documento original del proyecto.

## Desarrollo local

1. `make install`
2. `make server-dev`
3. `make client-dev`
4. `make dev` para levantar el stack Docker de desarrollo

### Variables del servidor

- `PORT` default `8080`
- `CLIENT_ORIGIN` default `http://localhost:3000`
- `REQUEST_TIMEOUT` default `20s`
- `NAVIGATION_TIMEOUT` default `15s`
- `NETWORK_IDLE_WAIT` default `1500ms`
- `VIEWPORT_WIDTH` default `1440`
- `VIEWPORT_HEIGHT` default `900`
- `BROWSER_BIN` ruta opcional a Chromium/Chrome
- `GREENCHECK_BASE_URL` default `https://api.thegreenwebfoundation.org/api/v3/greencheck`
- `AI_PROVIDER` default `rule_based`
- `GEMINI_API_KEY` opcional
- `GEMINI_MODEL` default `gemini-2.0-flash`
- `LLM_TIMEOUT` default `12s`

### Variables del cliente

- `SCANNER_API_URL` URL interna usada por el BFF de Next. En desarrollo, si falta, usa `http://localhost:8080`
- `SCANNER_SELF_BASE_URL` URL interna opcional para reescribir scans de la propia app cuando el origen público coincide exactamente (`scheme + host + port`)
- `APP_BASE_URL` URL pública de la app, usada también para resolver scans same-origin detrás de proxy

## Endpoints principales

- `GET /healthz` en Go
- `POST /api/v1/scans` en Go
- `POST /api/v1/green-fix` en Go
- `POST /api/scan` en Next
- `POST /api/green-fix` en Next
- `GET /api/healthz` en Next

## Producción y Dokploy

Archivos añadidos:

- `docker/client.prod.Dockerfile`
- `docker/server.prod.Dockerfile`
- `docker/compose.prod.yml`

Despliegue esperado:

- `client` público detrás del reverse proxy de Dokploy
- `server` privado en red interna
- `SCANNER_API_URL=http://server:8080`
- `SCANNER_SELF_BASE_URL=http://client:3000` para que el showcase pueda escanear la propia app sin confundir otros servicios del mismo host en puertos distintos

Comando recomendado para validar la topología:

```bash
docker compose -f docker/compose.prod.yml config
```

## Notas

- Si Gemini falla o no está configurado, Wattless sigue funcionando con fallback heurístico.
- Si Greencheck falla, `hosting_verdict` pasa a `unknown` y el informe sigue siendo válido.
- Los overlays visuales solo aparecen cuando un recurso puede mapearse a un nodo visible del DOM.

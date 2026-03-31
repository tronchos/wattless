# Wattless

> Reductor de entropía digital. Escanea una URL, mide bytes, CO₂, LCP y devuelve un informe con recomendaciones de código real.

Wattless audita sostenibilidad y rendimiento web para la [Hackatón CubePath 2026](https://github.com/midudev/hackaton-cubepath-2026). Lanza un Chromium headless contra una URL pública, captura tráfico de red, métricas de render, screenshot completo y genera un informe técnico con insights accionables.

![Dashboard de Wattless](docs/media/wattless-dashboard.webp)

## Desarrollo

```bash
make install
make dev           # http://localhost:5173
```

El servidor Vite corre en `:5173` y hace proxy de `/api` y `/healthz` hacia el backend Go en `:18080`.

## Producción

```bash
make prod          # http://localhost:8080
```

Un solo binario Go sirve el frontend embebido, la API y el scanner. Sin Node runtime, sin CORS, sin coordinación de procesos.

También puedes desplegar frontend y backend por separado. El frontend ya soporta `VITE_API_BASE_URL` para apuntar a una API remota y el backend acepta `CLIENT_ORIGIN` para CORS cross-domain.

## Build manual

```bash
make build         # Compila frontend + backend en server/bin/wattless
```

## Tests

```bash
make test          # Go tests + Vitest
```

## Arquitectura

```
Browser → Go binary (SPA estática + API + scanner)
               ↓
        Chromium headless (rod)
               ↓
       Análisis + insights → JSON + screenshot binario
```

- `client/` — Vite + React + Tailwind. Build estático embebido en el binario Go con `go:embed`.
- `server/` — Go. Scanner con rod, análisis de recursos, cálculo de CO₂, insights IA con fallback heurístico.
- `docker/` — Compose para dev y producción.

### Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/healthz` | Health check |
| `POST` | `/api/v1/scans` | Encolar un scan |
| `GET` | `/api/v1/scans/{id}` | Estado y reporte del scan |
| `GET` | `/api/v1/scans/{id}/screenshot` | Screenshot binario (tiles) |
| `GET` | `/*` | Frontend SPA (fallback a index.html) |

## Fórmula CO₂

```
(bytes / 1_000_000_000) × 0.8 × 0.75 × 442 = gCO₂/visita
```

| Factor | Valor | Fuente |
|--------|-------|--------|
| Intensidad energética | 0.8 kWh/GB | Sustainable Web Design |
| Factor de retorno | 0.75 | Visitas repetidas |
| Intensidad de carbono | 442 gCO₂e/kWh | Promedio global |

## Variables de entorno

### Backend

| Variable | Default | Descripción |
|----------|---------|-------------|
| `PORT` | `8080` | Puerto del servidor |
| `CLIENT_ORIGIN` | `http://localhost:5173` | Origins permitidos (CORS, separados por coma) |
| `BROWSER_BIN` | — | Ruta a Chromium |
| `REQUEST_TIMEOUT` | `20s` | Timeout general |
| `NAVIGATION_TIMEOUT` | `15s` | Timeout de navegación |
| `VIEWPORT_WIDTH` | `1440` | Ancho del viewport |
| `VIEWPORT_HEIGHT` | `900` | Alto del viewport |
| `AI_PROVIDER` | `rule_based` | `rule_based` o `gemini` |
| `GEMINI_API_KEY` | — | API key de Gemini |
| `GEMINI_MODEL` | `gemini-2.0-flash` | Modelo de Gemini |

### Frontend

| Variable | Default | Descripción |
|----------|---------|-------------|
| `VITE_PROXY_TARGET` | `http://localhost:8080` | Backend para el proxy del dev server |
| `VITE_API_BASE_URL` | — | Base pública de la API en producción. Usa el origen del backend, sin sufijo `/api`. |
| `VITE_PUBLIC_APP_URL` | — | URL pública para exportación Markdown y metadata social |

## Limitaciones

- El cálculo de CO₂ es una estimación basada en transferencia, no una medición eléctrica directa.
- El veredicto de hosting depende de la disponibilidad de The Green Web Foundation.
- Algunos recursos no tienen anclaje visual y no pueden resaltarse sobre la captura.
- El escáner solo acepta destinos públicos `http/https`; bloquea localhost, IPs privadas y hosts internos.

## Deploy separado en Dokploy

### Backend en Dokploy

- Tipo: Dockerfile
- Contexto: raíz del repo
- Dockerfile recomendado: `docker/server-prod.Dockerfile`
- Puerto: `8080`

Variables mínimas:

| Variable | Ejemplo |
|----------|---------|
| `CLIENT_ORIGIN` | `https://app.tudominio.com` |
| `AI_PROVIDER` | `rule_based` o `gemini` |
| `GEMINI_API_KEY` | `...` |
| `GEMINI_MODEL` | `gemini-2.0-flash` |
| `REQUEST_TIMEOUT` | `20s` |
| `NAVIGATION_TIMEOUT` | `15s` |

Notas:

- `CLIENT_ORIGIN` acepta varios orígenes separados por coma si necesitas preview environments.
- El backend debe publicarse bajo HTTPS si el frontend también usa HTTPS.
- Si usas Gemini, `AI_PROVIDER=gemini` requiere `GEMINI_API_KEY`.

### Frontend en Dokploy con Nixpacks

- Tipo: Nixpacks
- Directorio/app root: `client`
- El repo ya incluye `client/nixpacks.toml`
- Puerto: Dokploy inyecta `PORT` y el runtime lo respeta

Variables mínimas:

| Variable | Ejemplo |
|----------|---------|
| `VITE_API_BASE_URL` | `https://api.tudominio.com` |
| `VITE_PUBLIC_APP_URL` | `https://app.tudominio.com` |

Notas:

- `VITE_API_BASE_URL` debe apuntar al origen del backend, no a `/api`.
- El frontend sirve `dist` con fallback SPA y `GET /healthz`.
- Si prefieres Railpack, puedes reutilizar los mismos scripts: `npm ci`, `npm run build`, `npm run start`.

### Verificación recomendada

1. Abre el frontend en `https://app.tudominio.com`.
2. Envía un scan y confirma que `POST` y `GET` a `https://api.tudominio.com/api/v1/...` responden bien.
3. Revisa en DevTools que no haya errores de CORS ni mixed content.
4. Valida que el inspector visual cargue tiles desde `api.tudominio.com`.

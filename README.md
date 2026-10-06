# Relojería Yampier

Sitio web con backend en Go + MongoDB y frontend en React (Vite), más un panel
de administración para gestionar catálogo, inventario, consultas y reservas.

No es un e-commerce: no hay carrito, checkout ni pago online. El cliente ve el
reloj, consulta, el local coordina una reserva (con seña si corresponde) y el
pago final se hace en persona.

## Estructura

La arquitectura interna del backend (capas, carpetas, reglas y tests) está en
[`backend/README.md`](backend/README.md).

```
relojeria-yampier/
├── backend/     API en Go (net/http + MongoDB)
├── frontend/    Sitio público + panel Admin en React (Vite)
└── docker-compose.yml
```

**Stack:** React 18 + Vite + React Router (frontend), Go 1.22 con `net/http`
(backend), MongoDB 7, Docker Compose.

## Rutas principales

| Sitio público | |
|---|---|
| `/` | Inicio |
| `/relojes`, `/relojes/hombre`, `/relojes/mujer` | Catálogo (paginado) |
| `/reloj/:id` | Detalle con botón **Reservar** |
| `/reservar/:id` | Solicitud de reserva (requiere cuenta) |
| `/ingresar`, `/crear-cuenta` | Cuenta de cliente |
| `/recuperar-contrasena`, `/restablecer-contrasena` | Recuperación de contraseña |
| `/mi-cuenta`, `/mi-cuenta/reservas` | Datos y reservas del cliente |
| `/servicios`, `/contacto`, `/como-reservar` | Informativas |
| `/privacidad`, `/terminos-de-reserva` | Legales (borradores) |

| Panel Admin | |
|---|---|
| `/admin/login` | Ingreso |
| `/admin` | Dashboard |
| `/admin/relojes`, `/admin/reservas`, `/admin/stock`, `/admin/clientes`, `/admin/consultas` | Gestión |

## Opción 1: correr todo con Docker (recomendado)

1. Copiá el archivo de entorno de la raíz y completá los valores:
   ```bash
   cp .env.example .env
   ```
   `JWT_SECRET` es obligatorio: el backend no arranca si está vacío (ni en Docker
   ni en local). Generá una clave larga, por ejemplo con `openssl rand -hex 32`.
   Definí también `ADMIN_EMAIL` y `ADMIN_PASSWORD` (se usan para crear el primer
   usuario del panel Admin). Para ver datos de demostración usá
   `APP_ENV=development` y `SEED_DEMO_DATA=true` (ver [Seed demo](#seed-demo)).
   Para un despliegue real: `APP_ENV=production` y `SEED_DEMO_DATA=false`.

2. Levantá todo:
   ```bash
   docker compose up --build
   ```
   Cada servicio tiene un healthcheck (Mongo, backend y frontend), así que
   `docker compose` espera a que Mongo esté realmente listo antes de levantar
   el backend, y a que el backend responda antes de levantar el frontend.

3. Abrí:
   - Sitio: http://localhost:5173
   - Panel Admin: http://localhost:5173/admin/login (con el email y password del `.env`)
   - Estado de la API: http://localhost:8080/api/health

Los datos de MongoDB y las imágenes subidas quedan en volúmenes de Docker
(`mongo-data`, `backend-uploads`), separados del código: no se suben a git.

## Opción 2: correr en desarrollo, sin Docker

Necesitás **Go 1.22+**, **Node 18+** y **MongoDB** corriendo en `localhost:27017`
(podés instalarlo local o levantar solo el contenedor de Mongo: `docker compose up mongo`).

**Backend:**
```bash
cd backend
cp .env.example .env      # completá JWT_SECRET, ADMIN_EMAIL, ADMIN_PASSWORD
go mod tidy                # descarga las dependencias y genera go.sum
go run ./cmd/api
```
El backend carga `backend/.env` automáticamente (con [godotenv](https://github.com/joho/godotenv))
al ejecutar `go run ./cmd/api`, así que con copiar el archivo alcanza — no hace falta
exportar las variables a mano en la terminal. En Docker esto no aplica: ahí
las variables llegan directo del `environment:` de `docker-compose.yml`.

**Frontend:**
```bash
cd frontend
cp .env.example .env
npm install
npm run dev
```

## Panel Admin

- `/admin/login`: iniciar sesión (lleva al Dashboard).
- `/admin` (Inicio): reservas pendientes, confirmadas y con seña, consultas
  nuevas, relojes sin stock y con última unidad (cada número abre la vista
  filtrada) y reservas que vencen en las próximas 48 h.
- `/admin/relojes`: catálogo con buscador (nombre, marca o modelo: `ga2100`
  encuentra `GA-2100-1A1`), filtros de stock y activos/archivados, orden por
  nombre/precio, contadores (total, sin stock, última unidad), ajuste rápido
  de stock (−/+), archivar/restaurar (con advertencia si tiene reservas
  activas), historial de stock por reloj y alta/edición con subida de imagen.
- `/admin/stock`: movimientos de stock filtrables por reloj, tipo y fecha.
- `/admin/clientes`: cuentas de clientes; desactivar/reactivar (nunca se borran).
- `/admin/reservas`: reservas con filtros por estado, reloj y fecha; confirmar,
  registrar seña (monto y método), completar, cancelar, notas, vencimiento,
  fecha de cada cambio de estado y datos completos del cliente.
- `/admin/consultas`: consultas con estado (nueva / contactado / cerrada),
  notas internas, reloj que la originó y acceso directo a "Crear reserva".

El primer usuario admin se crea automáticamente al arrancar el backend, usando
`ADMIN_EMAIL` y `ADMIN_PASSWORD`. Para agregar más usuarios admin más adelante,
se puede insertar directamente en la colección `admins` de MongoDB (con la
contraseña hasheada con bcrypt) o agregar un endpoint de registro protegido.

## Catálogo e inventario

- El catálogo público muestra por ahora solo relojes **Casio**
  (`CATALOG_BRAND` en `frontend/src/utils/siteConfig.js`) y solo relojes activos.
- **Modelo único**: cada reloj tiene un `model` obligatorio (ej. `GA-2100-1A1`),
  guardado en mayúsculas. Para detectar duplicados se compara `modelKey`
  (solo letras y números: `GA-2100-1A1` = `ga 2100 1a1`), con índice único en
  Mongo. Si al arrancar ya hubiera duplicados, el backend lo informa en el log
  y no crea el índice (no borra nada); la validación previa igual evita nuevos.
- **Precio**: `priceARS`, entero en pesos (sin centavos ni punto flotante).
  `0` = "Consultar precio".
- **Archivado**: los relojes no se borran en el flujo normal; se archivan
  (`archivedAt`) y dejan de verse en el sitio, conservando consultas y reservas.
  El borrado definitivo es excepcional: solo un reloj archivado y sin reservas.

### Modelo de stock

`stockQuantity` (entero >= 0) es la única fuente de verdad. La disponibilidad
que ve el cliente se deriva y no se guarda:

| stockQuantity | Se muestra como |
|---|---|
| 0 | Sin stock (visible, sin acción de reserva) |
| 1 | Última unidad |
| 2 o más | En stock |

Una **consulta nunca descuenta stock**. El stock cambia solo por: alta del
reloj, ajustes del Admin y reservas (ver abajo). El backend rechaza cualquier
operación que lo dejaría negativo.

### Movimientos de stock (trazabilidad)

Cada cambio de `stockQuantity` genera una línea en `stock_movements` con
tipo, cantidad, stock anterior y posterior (tomados de la misma operación
atómica que cambió el stock), reserva asociada, motivo y quién lo hizo.

| Tipo | Cuándo | Cantidad |
|---|---|---|
| `stock_entry` | Alta de un reloj con stock, o ingreso de mercadería | +n |
| `manual_adjustment` | Ajuste desde el Admin (−/+ o formulario) | ±n |
| `correction` | Corrección de inventario (elegible al editar) | ±n |
| `reservation_hold` | Reserva confirmada | −1 |
| `reservation_release` | Reserva confirmada que se cancela o vence | +1 |
| `sale` | Reserva completada | 0 (la unidad ya se había descontado al confirmar) |

Los movimientos de reservas llevan una clave única (tipo + reserva): un
reintento nunca duplica una línea. Sin transacciones, el movimiento se anota
después de que el cambio quedó firme; si el proceso se cayera justo en el
medio, faltaría la línea (se nota como un salto entre un `stockAfter` y el
siguiente `stockBefore`), pero nunca se anota un movimiento que no ocurrió.

## Reservas

Colección `reservations`. Cada reserva guarda un **snapshot** del reloj
(`watchNameSnapshot`, `watchModelSnapshot`, `priceAtReservation`): si después
cambia el precio o el nombre del reloj, la reserva conserva lo acordado.

| Estado | Significado | Stock |
|---|---|---|
| `pending` | Pedido registrado, sin confirmar | No retiene |
| `confirmed` | El local confirmó la reserva | **Retiene 1 unidad** |
| `deposit_paid` | Seña recibida | Sigue retenida |
| `completed` | Pagado y retirado en el local | Unidad vendida |
| `cancelled` | Cancelada | Si retenía, la devuelve |
| `expired` | Venció sin completarse | Si retenía, la devuelve |

Transiciones permitidas: `pending → confirmed | cancelled | expired`,
`confirmed → deposit_paid | completed | cancelled | expired`,
`deposit_paid → completed | cancelled`. Los estados finales no se reabren
(cualquier otra transición responde 409).

**Concurrencia (última unidad)**: retener una unidad es una única operación
condicional de Mongo (`{stockQuantity: {$gte: 1}}` + `$inc: -1`), así dos
confirmaciones simultáneas no pueden quedarse con la misma unidad: una recibe
409. Los cambios de estado también son condicionales al estado leído, así una
unidad nunca se devuelve dos veces. Mongo corre sin replica set (sin
transacciones), por eso el orden de los pasos está elegido para que un corte
a mitad de operación solo pueda dejar el stock por debajo del real (corregible
desde el Admin), nunca reservar de más. Hay tests de integración que lo prueban.

**Fechas de estado**: `confirmedAt`, `depositPaidAt`, `completedAt`,
`cancelledAt` y `expiredAt` se guardan al cambiar de estado (las reservas
anteriores a este cambio no los tienen).

**Vencimiento** (`expiresAt`, opcional, máx. 90 días): solo vencen reservas
`pending` o `confirmed`; las que tienen seña paga las resuelve el local. Se
aplica de tres formas, sin infraestructura extra: al listar reservas en el
Admin, antes de cada cambio de estado, y con una rutina en segundo plano cada
`RESERVATION_SWEEP_INTERVAL` (5 min por defecto).

## Cuentas de clientes y reservas web

Flujo: el cliente abre un reloj → **Reservar** → ingresa o crea su cuenta
(vuelve al mismo reloj vía `?returnTo=`, validado contra open redirects) →
acepta los Términos de Reserva → la solicitud queda **pendiente** (no
descuenta stock) → el Admin confirma (recién ahí se retiene la unidad, con la
operación atómica de siempre) → seña si corresponde → pago y retiro en el local.

- **Cuentas separadas del Admin**: colección `customers`, login en
  `/api/auth/login`, token con rol `customer`. Los tokens llevan rol: un token
  de cliente recibe 403 en `/api/admin/*` y viceversa. Contraseñas con bcrypt.
- **Datos pedidos** (minimización): nombre, apellido, email, teléfono, tipo y
  número de documento, condición fiscal; CUIT/CUIL/CDI obligatorio solo para
  monotributo, responsable inscripto y exento (se valida el dígito
  verificador); domicilio de facturación opcional. No se piden fecha de
  nacimiento, género ni imágenes de documentos.
- **Consentimientos**: privacidad obligatoria (`privacyAcceptedAt`,
  `privacyVersion`); promociones opcional y revocable (`marketingConsent`,
  `marketingConsentAt`); términos al reservar (`termsAcceptedAt`,
  `termsVersion`). Las versiones están en `backend/internal/domain/customer.go` y
  `frontend/src/utils/legal.js` (actualizar ambas si cambia un texto).
- **Privacidad de datos**: un cliente solo ve sus reservas (una reserva ajena
  responde 404); la vista del cliente no incluye notas internas; los datos
  fiscales y el domicilio solo se ven en el Admin. El login responde igual
  si el email no existe o la contraseña es incorrecta.
- **Cancelación por el cliente**: solo solicitudes pendientes. Confirmadas o
  con seña se cancelan contactando al local (puede haber una seña de por medio).
- **Seña**: registro administrativo de monto, método (`cash`, `bank_transfer`,
  `in_store_card`, `other`) y fecha. No hay cobro online.
- Una solicitud web pendiente vence sola a los 7 días; máximo 3 solicitudes
  pendientes por cliente y una activa por reloj.
- Páginas `/privacidad` y `/terminos-de-reserva`: **borradores** marcados para
  revisión legal antes de producción.

## Contacto del local

- Dirección, teléfono y horarios del local: `frontend/src/utils/siteConfig.js`.
- WhatsApp, email y redes: variables `VITE_WHATSAPP_NUMBER`, `VITE_CONTACT_EMAIL`,
  `VITE_INSTAGRAM_URL`, `VITE_FACEBOOK_URL` en `frontend/.env` (desarrollo) o en
  el `.env` de la raíz (Docker, requiere `docker compose up --build`). Si están
  vacías, esos botones no se muestran.
- La página pública `/como-reservar` explica el proceso de reserva
  (`/mi-reserva`, ruta vieja, redirige ahí).

## Seed demo

`SEED_DEMO_DATA=true` carga 6 relojes Casio (especificaciones de casio.com,
imágenes en `frontend/public/images/relojes/casio/`, precio 0 = a consultar).

- Se carga **una sola vez por base**: queda una marca en la colección
  `app_meta`. Aunque después se archiven o borren esos relojes, reiniciar el
  backend no los vuelve a insertar.
- Con `APP_ENV=production` nunca se cargan, aunque `SEED_DEMO_DATA` quede en
  `true` por error. En Docker, si `APP_ENV` no está definido, vale `production`.
- Desarrollo: `APP_ENV=development` + `SEED_DEMO_DATA=true`.
  Producción: `APP_ENV=production` + `SEED_DEMO_DATA=false`.

## Datos existentes (compatibilidad)

Al arrancar, el backend completa campos nuevos en documentos viejos, de forma
**aditiva e idempotente** (no borra ni pisa nada; `price`, `stock` y `read`
quedan como estaban):

- `priceARS` ← `price` redondeado a pesos.
- `stockQuantity` ← `stock`: `sin_stock` → 0, `ultima_unidad`/`en_stock` → 1,
  sin dato → 0. Es conservador a propósito (el estado viejo no decía cuántas
  unidades había): **revisá las cantidades reales desde el Admin**.
- `brandKey` / `modelKey` ← derivados de `brand` / `model`.
- Consultas: `status` ← `contacted` si estaban leídas, si no `new`.
- Campos agregados después (`tokenVersion`, fechas de estado de reservas,
  `contactedAt`/`closedAt`, `watchModelSnapshot`) no requieren migración:
  si faltan, valen 0 / vacío, y los tokens emitidos antes siguen válidos hasta
  que la cuenta cambie de contraseña.

## API (resumen)

Públicas: `GET /api/health`, `GET /api/watches`, `GET /api/watches/{id}`,
`POST /api/inquiries` (acepta `watchId` opcional), `POST /api/admin/login`,
`POST /api/auth/register`, `POST /api/auth/login`.

Recuperación de contraseña (públicas, con rate limit): `POST /api/auth/password/forgot`,
`POST /api/auth/password/reset`.

Cliente (JWT rol `customer`): `GET|PUT /api/auth/me`, `POST /api/auth/me/password`,
`POST|GET /api/reservations`, `GET /api/reservations/{id}`, `POST /api/reservations/{id}/cancel`.

Catálogo paginado:
```
GET /api/watches?page=1&limit=24&search=ga2100&brand=casio&gender=hombre&availability=in_stock&sort=price_asc
→ { "items": [...], "page": 1, "limit": 24, "total": 120, "pages": 5 }
```
`availability`: `in_stock` (>= 2), `last_unit` (1), `out_of_stock` (0),
`available` (>= 1). `sort`: `newest`, `name_asc`, `name_desc`, `price_asc`,
`price_desc`. `limit` máximo 100 (si se excede: 400).

Admin (JWT rol `admin`): `/api/admin/dashboard`, `/api/admin/watches` (+ `/{id}`,
`/{id}/stock`, `/{id}/archive[?confirm=true]`, `/{id}/restore`),
`/api/admin/stock-movements`, `/api/admin/reservations` (+ `/{id}`, `/{id}/status`),
`/api/admin/inquiries` (+ `/{id}`), `/api/admin/customers` (+ `/{id}/deactivate`,
`/{id}/reactivate`), `/api/admin/uploads`.

**Formato de error** (toda la API, incluidas rutas `/api/` inexistentes):
```
{ "error": "Mensaje para mostrar", "code": "OUT_OF_STOCK" }
```
`error` se mantiene como texto (compatible con versiones anteriores); `code`
es estable para que el cliente reaccione sin comparar textos. Genéricos:
`INVALID_INPUT` (400), `UNAUTHENTICATED`/`SESSION_EXPIRED` (401), `FORBIDDEN`
(403), `NOT_FOUND` (404), `CONFLICT` (409), `RATE_LIMITED` (429),
`INTERNAL_ERROR` (500, el detalle solo va al log). Específicos:
`RESERVATION_NOT_FOUND`, `WATCH_NOT_FOUND`, `OUT_OF_STOCK`,
`INVALID_TRANSITION`, `CONCURRENT_UPDATE`, `DUPLICATE_MODEL`,
`ACTIVE_RESERVATIONS` (incluye `activeReservations`), `INVALID_RESET_TOKEN`,
`INVALID_CREDENTIALS`.

## Observabilidad

- `GET /api/health` → `{"status":"ok","database":"ok"}` (200) o
  `{"status":"degraded","database":"unavailable"}` (503). No expone URI ni datos internos.
- Cada request se loguea en una línea estructurada (`log/slog`):
  `level=INFO msg=request method=GET path=/api/watches status=200 duration_ms=23 ip=… request_id=…`.
  Los panics se convierten en un 500 seguro (stack solo en el log).
  El request ID también vuelve en el header `X-Request-ID`. Nunca se loguean
  headers (JWT), cuerpos (contraseñas, datos personales) ni query strings.

## Sesiones

- **Tokens**: JWT firmados con `JWT_SECRET` (HMAC), con rol (`admin` /
  `customer`) y `tokenVersion`. Admin: 24 h. Cliente: 72 h.
- **Invalidación**: cada cuenta guarda `tokenVersion`; el backend la compara en
  cada request. Sube al cambiar o restablecer la contraseña y al desactivar
  una cuenta → todos los tokens anteriores dejan de valer (401
  `SESSION_EXPIRED`). Al cambiar la contraseña, la sesión actual recibe un
  token nuevo.
- **Almacenamiento**: hoy el frontend guarda el token en `localStorage`
  (claves separadas para Admin y cliente). Es simple y funciona con frontend y
  API en orígenes distintos, pero un XSS podría leerlo. Mitigaciones actuales:
  React escapa todo el contenido, no se usa `dangerouslySetInnerHTML`, TTL
  acotados e invalidación por `tokenVersion`.
- **Migración a cookies HttpOnly (recomendada antes de producción)**, no
  implementada a medias a propósito porque requiere cambios coordinados:
  1. Servir frontend y API bajo el mismo sitio (ej. `yampier.com` y
     `yampier.com/api` con Nginx como proxy), así la cookie es `SameSite=Lax`
     o `Strict` sin CORS con credenciales.
  2. En login, setear `Set-Cookie: session=<jwt>; HttpOnly; Secure; SameSite=Lax; Path=/api`
     y un endpoint de logout que la borre.
  3. En `requireAuth`/`requireCustomer`, leer el token de la cookie además del
     header `Authorization` (convivencia durante la transición).
  4. Protección CSRF para métodos que modifican (token CSRF de doble envío o
     verificación estricta de `Origin`).
  5. Quitar `localStorage` del frontend (`services/api.js`) y usar
     `credentials: 'include'`.

## Recuperación de contraseña (clientes)

`/recuperar-contrasena` → el cliente ingresa su email → respuesta siempre
igual (no revela si la cuenta existe) → se genera un token aleatorio de 32
bytes (no es un JWT), se guarda **solo su hash SHA-256**, vence a los 30
minutos y es de un solo uso → email con enlace a
`APP_PUBLIC_URL/restablecer-contrasena?token=…` (la página quita el token de
la barra de direcciones) → nueva contraseña → token consumido y todas las
sesiones cerradas. Pedir un enlace nuevo invalida los anteriores. Rate limit:
3 pedidos cada 15 min por IP. Mongo borra solos los pedidos un día después de
vencidos (índice TTL).

**Envío de emails** (`MAIL_DRIVER`):
- vacío: no se envía nada (el flujo responde igual; queda un aviso en el log).
- `log`: **solo desarrollo**, escribe el email con el enlace en el log del
  backend. Se rechaza con `APP_ENV=production`.
- `smtp`: `SMTP_HOST`, `SMTP_PORT` (587), `SMTP_USERNAME`, `SMTP_PASSWORD`,
  `MAIL_FROM`. Usa STARTTLS. Completar con las credenciales del proveedor de
  correo del local (no hay ninguna configurada en el proyecto).

## Clientes: desactivación

Las cuentas no se borran (tienen reservas históricas). Desde `/admin/clientes`
se desactivan: no pueden iniciar sesión, sus sesiones se cierran
(`tokenVersion`) y su historial se conserva. Se pueden reactivar.

## Backups de MongoDB

Estrategia mínima recomendada:

- **Qué**: la base `relojeria_yampier` completa (catálogo, reservas, clientes,
  consultas, movimientos) y el volumen de imágenes subidas (`backend-uploads`).
- **Frecuencia**: diaria (de noche) + antes de cada actualización del sistema.
  Conservar al menos 7 diarios y 4 semanales, **fuera del servidor**.
- **Verificación**: una vez por mes, restaurar un backup en una base de prueba
  y revisar que los conteos coincidan. Un backup que nunca se restauró no está
  verificado.
- Los backups contienen datos personales: guardarlos cifrados y con acceso restringido.

Comandos de ejemplo (Docker, desarrollo/local):

```bash
# Backup (archivo comprimido con fecha)
docker exec yampier-mongo mongodump --db relojeria_yampier --archive --gzip > backup-$(date +%F).archive.gz

# Verificación: restaurar en una base de PRUEBA (no toca la real)
docker exec -i yampier-mongo mongorestore --archive --gzip \
  --nsFrom='relojeria_yampier.*' --nsTo='relojeria_restore_test.*' < backup-AAAA-MM-DD.archive.gz
docker exec yampier-mongo mongosh --quiet relojeria_restore_test --eval 'db.getCollectionNames().map(c => c + ": " + db[c].countDocuments())'

# Imágenes subidas
docker run --rm -v relojeria-yampier_backend-uploads:/data -v "$PWD":/backup alpine tar czf /backup/uploads-$(date +%F).tgz -C /data .
```

Para restaurar sobre la base real, primero hacer un backup del estado actual
y restaurar con el sistema detenido. No usar `--drop` sin estar seguro.

## Rate limiting detrás de un proxy

El rate limiting (logins, registro, recuperación de contraseña, consultas y
solicitudes de reserva) usa la IP de la conexión. Por defecto
**ignora `X-Forwarded-For`**, porque cualquier cliente puede falsearlo. Si
ponés el backend detrás de un Nginx propio:

1. En Nginx: `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`
2. En el backend: `TRUSTED_PROXIES` con la IP o rango del proxy
   (ej. `TRUSTED_PROXIES=172.16.0.0/12` en la red de Docker).

Con eso el backend toma la IP más a la derecha de `X-Forwarded-For` que no sea
un proxy confiable (las entradas de la izquierda las puede inventar el cliente).
No pongas rangos amplios ni públicos en `TRUSTED_PROXIES`.

## Variables de entorno del backend

| Variable | Default | Uso |
|---|---|---|
| `JWT_SECRET` | — (obligatoria) | Firma de sesiones (Admin y clientes). Nunca en el frontend |
| `APP_ENV` | vacío (Docker: `production`) | `production` bloquea el seed demo |
| `SEED_DEMO_DATA` | `false` | Carga demo única (solo fuera de producción) |
| `TRUSTED_PROXIES` | vacío | Proxies cuyo `X-Forwarded-For` se acepta |
| `RESERVATION_SWEEP_INTERVAL` | `5m` | Frecuencia del vencimiento automático |
| `MONGO_URI` / `MONGO_DB` | localhost / `relojeria_yampier` | Conexión |
| `APP_PUBLIC_URL` | `http://localhost:5173` | URL del sitio para los enlaces de los emails |
| `MAIL_DRIVER` | vacío | `smtp`, `log` (solo desarrollo) o vacío (sin envío) |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `MAIL_FROM` | vacío / 587 | Envío SMTP |
| `ALLOWED_ORIGIN`, `PORT`, `UPLOADS_DIR`, `ADMIN_EMAIL`, `ADMIN_PASSWORD` | | Sin cambios |

## Calidad: lint, formato y tests

**Frontend:**
```bash
cd frontend
npm run lint          # ESLint
npm run format:check  # Prettier (solo verifica, no reescribe)
npm run format         # Prettier (reescribe)
npm test               # Vitest
npm run build           # build de producción
```

**Backend:**
```bash
cd backend
gofmt -w .
go vet ./...
go test ./...
```
Los tests de lógica pura (validaciones, transiciones, JWT, proxies) corren
siempre. Los de integración (reservas concurrentes sobre la última unidad,
cancelación/vencimiento que liberan stock, movimientos de stock, modelo
duplicado, flujo HTTP de clientes, tokenVersion, recuperación de contraseña,
autorización por dueño) usan MongoDB en `localhost:27017` o en `MONGO_TEST_URI`: crean una
base temporal `yampier_test_<id>` y la borran al terminar (nunca tocan la base
real). Si no hay Mongo disponible, se saltean con un aviso (`go test -v` lo
muestra como SKIP). Para correrlos: `docker compose up -d mongo`.

## Seguridad — resumen de lo que ya está cubierto

- `JWT_SECRET` obligatorio: sin clave por defecto insegura.
- Tokens verificados exigiendo el algoritmo HMAC esperado (evita confusión de algoritmo).
- Subida de imágenes: se valida el contenido real del archivo (`http.DetectContentType`),
  no solo la extensión del nombre; el nombre final siempre lo genera el servidor.
- Rate limiting en memoria sobre `POST /api/admin/login` y `POST /api/inquiries`
  (ver limitación de instancia única documentada en `backend/internal/middleware/ratelimit.go`), con
  soporte opcional de proxy confiable (`TRUSTED_PROXIES`).
- Toda validación importante (precio, stock, modelo, estados, IDs, longitudes,
  seña, fechas) vive en el backend, no solo en el frontend.
- Búsquedas: el texto del usuario se escapa (`regexp.QuoteMeta`) antes de usarse
  en Mongo; `sort`, `availability` y `status` solo aceptan valores de una lista.
- CORS con un único origen explícito (`ALLOWED_ORIGIN`), nunca `*`.
- Roles en el token y `tokenVersion` verificada contra la base en cada
  request; autorización por dueño en reservas de clientes (una ajena da 404).
- Contraseñas con bcrypt; tokens de recuperación aleatorios, guardados como hash.
- Valores escritos por usuarios en pipelines de Mongo van envueltos en
  `$literal` (un texto que empiece con `$` no se interpreta como campo).
- `VITE_*` solo contiene datos públicos (URL de la API, contacto): Vite los
  incrusta en el bundle. Ningún secreto va con prefijo `VITE_`.

## Subir a GitHub

El `.gitignore` y los `.dockerignore` ya excluyen `.env`, `node_modules`, `dist`
y las imágenes subidas en local. Nunca se sube ninguna credencial ni los datos
de MongoDB: solo el código y los `docker-compose.yml` / `Dockerfile` que arman
el entorno. Cada quien que clone el repo copia su propio `.env` a partir de
los `.env.example` (`/.env.example`, `backend/.env.example`, `frontend/.env.example`).

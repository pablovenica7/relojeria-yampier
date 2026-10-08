# Publicar Relojería Yampier en Internet

Guía para pasar de "funciona en mi compu" a un link público con HTTPS.
El desarrollo local (`docker compose up` o `npm run dev` + `go run ./cmd/api`)
no cambia.

## Arquitectura de producción

```
Navegador ──HTTPS──▶ Render Static Site        (frontend React, CDN)
    │                https://relojeria-yampier.onrender.com
    │
    └──HTTPS──▶ Render Web Service (Docker)    (backend Go)
                https://relojeria-yampier-api.onrender.com/api
                        │
                        └──TLS──▶ MongoDB Atlas (datos + imágenes en GridFS)
```

- **Un solo proveedor para frontend y backend (Render)**: un panel, un archivo
  (`render.yaml`) que describe todo, deploy automático en cada push a `main`,
  HTTPS automático, variables secretas por servicio y soporte de Docker para Go.
- **MongoDB Atlas** para la base: gestionada, con TLS, accesible desde el hosting.
- **Imágenes en MongoDB (GridFS)**: el disco de los contenedores del hosting se
  borra en cada deploy. Por eso las fotos subidas desde el Admin se guardan en
  la base. No hace falta un disco pago ni otra cuenta (Cloudinary/S3), y las
  imágenes entran en los mismos backups que los datos. Las URLs siguen siendo
  `/uploads/<archivo>`, así que el frontend no cambió.

### Costos y limitaciones

| Servicio | Plan | Costo | Limitación relevante |
|---|---|---|---|
| Render Static Site (frontend) | Free | US$0 | Ninguna importante para este sitio |
| Render Web Service (backend) | Free | US$0 | **Se duerme tras ~15 min sin visitas**: la primera visita después tarda ~1 min en mostrar el catálogo |
| Render Web Service (backend) | Starter | ~US$7/mes | Siempre encendido (recomendado cuando el sitio esté en uso real) |
| MongoDB Atlas | M0 Free | US$0 | 512 MB (sobra para catálogo, reservas y fotos de hasta 5 MB), **sin backups automáticos** |
| Dominio `.com` | — | ~US$10–15/año | Opcional |

Para pasar el backend a Starter: en Render → servicio `relojeria-yampier-api` →
Settings → Instance Type → Starter (o cambiar `plan: free` por
`plan: starter` en `render.yaml`).

> **¿Y Vercel?** Sirve igual para el frontend, pero su plan gratuito (Hobby) es
> solo para uso no comercial, y el backend Go igual necesitaría otro proveedor.
> Con Render queda todo en un lugar.

---

## Paso 1 — MongoDB Atlas (manual)

1. Entrá a https://www.mongodb.com/cloud/atlas/register y creá la cuenta.
2. **Crear cluster**: *Create* → plan **M0 (Free)** → proveedor **AWS** →
   región **N. Virginia (us-east-1)**, la misma zona que el backend en Render
   (menos latencia). Nombre: `yampier` → *Create Deployment*.
3. **Usuario de la base**: en el diálogo que aparece (o *Database Access* →
   *Add New Database User*):
   - Método: *Password*. Usuario: `yampier-app`.
   - *Autogenerate Secure Password* → **copiala a un lugar seguro**
     (gestor de contraseñas). Conviene una autogenerada: solo letras y números,
     así no hay que codificar caracteres especiales en la URI.
   - Privilegios: *Specific Privileges* → `readWrite` sobre la base
     `relojeria_yampier` (no hace falta `atlasAdmin`).
4. **Acceso de red**: *Network Access* → *Add IP Address* →
   *Allow access from anywhere* (`0.0.0.0/0`).
   Render (plan free/starter) no tiene IP de salida fija, así que es necesario.
   La protección es el usuario con contraseña fuerte, con permisos solo
   sobre esta base, y la conexión cifrada con TLS.
5. **Connection string**: *Database* → *Connect* → *Drivers* → Go. Copiá algo como:
   ```
   mongodb+srv://yampier-app:<db_password>@yampier.xxxxx.mongodb.net/?retryWrites=true&w=majority&appName=yampier
   ```
   Reemplazá `<db_password>` por la contraseña del paso 3. Ese es tu `MONGO_URI`.
   **No lo pegues en ningún archivo del proyecto**: va solo en Render (paso 3).

## Paso 2 — Código en GitHub

Los cambios de esta preparación tienen que estar en `main`:

```bash
git add -A
```
```bash
git status
```
```bash
git commit -m "chore(deploy): Render + MongoDB Atlas, imágenes en GridFS"
```
```bash
git push origin main
```

## Paso 3 — Render (manual)

1. Entrá a https://dashboard.render.com/register y registrate **con GitHub**.
   Cuando GitHub pregunte, autorizá el acceso **solo** al repositorio
   `pablovenica7/relojeria-yampier`.
2. *New* → **Blueprint** → elegí el repo `relojeria-yampier` → rama `main`.
   Render lee `render.yaml` y muestra los dos servicios:
   `relojeria-yampier-api` (backend) y `relojeria-yampier` (frontend).
3. Completá las variables que pide (son las marcadas `sync: false`):

   **Backend `relojeria-yampier-api`**

   | Variable | Valor |
   |---|---|
   | `MONGO_URI` | la connection string de Atlas (paso 1.5) |
   | `ALLOWED_ORIGIN` | `https://relojeria-yampier.onrender.com` |
   | `APP_PUBLIC_URL` | `https://relojeria-yampier.onrender.com` |
   | `ADMIN_EMAIL` | el email con el que vas a entrar al Admin |
   | `ADMIN_PASSWORD` | una contraseña fuerte nueva (no la reutilices de otro lado) |

   `JWT_SECRET` no se pide: Render genera un valor aleatorio largo (`generateValue`).
   `APP_ENV=production`, `SEED_DEMO_DATA=false`, `MONGO_DB=relojeria_yampier` y
   `TRUSTED_PROXIES` ya vienen en `render.yaml`. `PORT` lo pone Render.

   **Frontend `relojeria-yampier`**

   | Variable | Valor |
   |---|---|
   | `VITE_API_URL` | `https://relojeria-yampier-api.onrender.com/api` |
   | `VITE_WHATSAPP_NUMBER` | número confirmado, solo dígitos (ej. `549351XXXXXXX`), o vacío |
   | `VITE_CONTACT_EMAIL` | email público del local, o vacío |
   | `VITE_INSTAGRAM_URL` / `VITE_FACEBOOK_URL` | URLs oficiales, o vacío |

4. *Apply*. El primer deploy tarda unos minutos (build de Go y de Vite).
5. **Verificá las URLs reales**: si el nombre estaba tomado, Render agrega un
   sufijo (ej. `relojeria-yampier-abcd.onrender.com`). Si pasó eso:
   - backend → *Environment*: corregí `ALLOWED_ORIGIN` y `APP_PUBLIC_URL` → *Save*.
   - frontend → *Environment*: corregí `VITE_API_URL` → *Save* → *Manual Deploy*
     → *Deploy latest commit* (Vite incrusta la URL al compilar).

El build del frontend **falla a propósito** si `VITE_API_URL` falta o no es
HTTPS. Así nunca se publica un sitio que llame a `localhost` o que tenga
contenido mixto.

### JWT_SECRET manual (solo si no usás el que genera Render)

```bash
node -e "console.log(require('crypto').randomBytes(48).toString('base64url'))"
```

Pegalo solo en Render → backend → *Environment*. Si lo cambiás, todas las
sesiones abiertas se cierran (es la forma de invalidarlas en caso de duda).

## Paso 4 — Verificación

1. `https://relojeria-yampier-api.onrender.com/api/health` →
   `{"database":"ok","status":"ok"}`.
2. Abrí `https://relojeria-yampier.onrender.com`, entrá directo (pegando la URL) a
   `/relojes`, `/contacto`, `/ingresar`, `/mi-cuenta` y `/admin/login`, y
   refrescá cada una: tienen que cargar, no dar 404.
3. `/admin/login` con `ADMIN_EMAIL` / `ADMIN_PASSWORD` → cargá un reloj con foto.
4. Desde una ventana de incógnito: creá una cuenta de cliente, iniciá sesión y
   pedí una reserva. Confirmala desde el Admin.
5. En Render → backend → *Logs* tenés que ver líneas
   `msg=request ... ip=<IP pública>`. Si **todas** las requests muestran la misma
   IP (la del proxy de Render), avisame: hay que ajustar `TRUSTED_PROXIES` para
   que el rate limiting distinga visitantes.

### Checklist

- [ ] GitHub limpio (sin `.env` ni secretos)
- [ ] frontend publicado
- [ ] backend publicado
- [ ] Mongo Atlas conectado (`/api/health` → `database: ok`)
- [ ] HTTPS en ambos (candado, sin avisos de contenido mixto en la consola)
- [ ] CORS correcto (el sitio carga el catálogo; otro origen no puede usar la API)
- [ ] registro funcionando
- [ ] login funcionando
- [ ] catálogo funcionando
- [ ] reservas funcionando
- [ ] Admin funcionando
- [ ] imágenes funcionando (y siguen después de un redeploy)
- [ ] datos persistentes (siguen después de un redeploy)
- [ ] secrets fuera de GitHub
- [ ] seed demo desactivado (catálogo vacío hasta cargar relojes reales)

## Datos iniciales

Producción arranca **vacía** (sin relojes demo). Opciones:

- **Recomendado**: cargar los relojes reales desde el Admin.
- **Copiar el catálogo local a Atlas** (solo relojes e imágenes; no copia clientes
  ni reservas de prueba). En PowerShell, desde la raíz del proyecto:
  1. Levantá el stack local una vez con el código nuevo, para que las imágenes
     viejas del disco pasen a MongoDB:
     ```bash
     docker compose up -d --build
     ```
  2. Exportá la base local dentro del contenedor y copiala a tu carpeta:
     ```bash
     docker exec yampier-mongo mongodump --db relojeria_yampier --gzip --archive=/tmp/catalogo.archive.gz
     ```
     ```bash
     docker cp yampier-mongo:/tmp/catalogo.archive.gz .
     ```
  3. Restaurá **solo** relojes e imágenes en Atlas (reemplazá `TU_MONGO_URI`;
     no hace falta la contraseña en ningún archivo):
     ```bash
     docker run --rm -v "${PWD}:/backup" mongo:7 mongorestore --uri "TU_MONGO_URI" --gzip --archive=/backup/catalogo.archive.gz --nsInclude "relojeria_yampier.watches" --nsInclude "relojeria_yampier.uploads.files" --nsInclude "relojeria_yampier.uploads.chunks"
     ```
  4. Borrá `catalogo.archive.gz` (ya está ignorado por git, pero no hace falta guardarlo).

## Dominio propio (relojeriayampier.com)

Cuando tengas el dominio (comprado en NIC.ar, Namecheap, Cloudflare, etc.):

1. Render → **frontend** → *Settings* → *Custom Domains* → *Add* →
   `relojeriayampier.com`. Render agrega también `www.relojeriayampier.com` y
   redirige uno al otro.
2. Render → **backend** → *Custom Domains* → *Add* → `api.relojeriayampier.com`.
3. En el panel DNS de donde compraste el dominio, creá **exactamente** los
   registros que Render muestra en cada dominio. Normalmente:

   | Tipo | Nombre | Valor |
   |---|---|---|
   | `A` | `@` (raíz) | la IP que indica Render (hoy `216.24.57.1`) |
   | `CNAME` | `www` | `relojeria-yampier.onrender.com` |
   | `CNAME` | `api` | `relojeria-yampier-api.onrender.com` |

   Borrá otros registros `A`/`AAAA` viejos de `@` si el registrador puso alguno
   por defecto. Render emite el certificado HTTPS solo, cuando el DNS propaga
   (minutos a pocas horas). Tocá *Verify* en Render.
4. Actualizá las variables:
   - backend: `ALLOWED_ORIGIN=https://relojeriayampier.com,https://www.relojeriayampier.com`
     y `APP_PUBLIC_URL=https://relojeriayampier.com`.
   - frontend: `VITE_API_URL=https://api.relojeriayampier.com/api` → *Save* →
     *Manual Deploy*.
   - Las fotos ya cargadas siguen funcionando: en la base se guardan como
     `/uploads/...` y el frontend les antepone la URL de la API vigente.

## Backups

Qué hay que proteger: `customers`, `reservations`, `watches`, `stock_movements`,
`inquiries`, `admins` y las imágenes (`uploads.files` / `uploads.chunks`). Todo
está en la misma base `relojeria_yampier`, así que **un solo dump lo cubre todo**.

- **Atlas M0 (gratis) no tiene backups automáticos.** Hacé un dump manual
  semanal y antes de cada cambio grande. En PowerShell:
  ```bash
  docker run --rm -v "${PWD}:/backup" mongo:7 mongodump --uri "TU_MONGO_URI" --db relojeria_yampier --gzip --archive=/backup/yampier-backup.archive.gz
  ```
  Renombralo con la fecha y guardalo **fuera** de la carpeta del proyecto
  (disco externo o nube personal, con acceso restringido: tiene datos personales).
- Probá restaurarlo una vez por mes en una base de prueba (ver la sección
  "Backups de MongoDB" del README). Un backup que nunca se restauró no está
  verificado.
- Cuando el negocio dependa del sistema, conviene pasar a un plan pago de Atlas
  (Flex o dedicado), que incluye snapshots automáticos. Revisá el precio actual
  en Atlas antes de cambiar.

## Logs

Render → servicio → *Logs*. El backend escribe en la salida estándar (no en
archivos), una línea por request:
`level=INFO msg=request method=GET path=/api/watches status=200 duration_ms=23 ip=… request_id=…`.
Nunca registra contraseñas, JWT, `JWT_SECRET`, `MONGO_URI`, tokens de
recuperación ni documentos. Los errores internos (Mongo, panics) van solo al
log; el visitante recibe un mensaje genérico.

## Emails (recuperación de contraseña)

En producción el envío está **desactivado** hasta configurar SMTP. Sin SMTP, el
formulario "olvidé mi contraseña" responde igual, pero no llega ningún email.
Para activarlo, agregá en el backend: `MAIL_DRIVER=smtp`, `SMTP_HOST`, `SMTP_PORT`,
`SMTP_USERNAME`, `SMTP_PASSWORD` (secreto) y `MAIL_FROM`. Usá un proveedor SMTP
(por ejemplo, la cuenta de correo del dominio o un servicio transaccional).

## Mantenimiento diario

- Cambiás código → `git push origin main` → Render compila y publica solo la
  parte que cambió (`backend/**` o `frontend/**`). Si el build o el health
  check fallan, Render **mantiene la versión anterior** en línea.
- Cambiar una variable `VITE_*` requiere *Manual Deploy* del frontend.
- `ADMIN_PASSWORD` solo se usa para **crear** el admin la primera vez. Cambiarla
  después en Render no cambia la contraseña de un admin que ya existe.

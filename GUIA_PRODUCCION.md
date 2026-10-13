# Guía de producción — Relojería Yampier

Cómo está armado el sitio publicado y dónde mirar cuando pasa algo.
Para publicar desde cero o conectar un dominio propio, ver [DEPLOY.md](DEPLOY.md).

| Qué | Dónde |
|---|---|
| Sitio público | https://relojeria-yampier.onrender.com/ |
| Panel Admin | https://relojeria-yampier.onrender.com/admin |
| Estado del servidor | https://relojeria-yampier-api.onrender.com/api/health |
| Panel de Render | https://dashboard.render.com |
| Panel de MongoDB Atlas | https://cloud.mongodb.com |
| Código | https://github.com/pablovenica7/relojeria-yampier |

---

## 1. Qué vive en cada lugar

| Cosa | Dónde vive | Ejemplo |
|---|---|---|
| **Código** (el programa) | **GitHub** | Pantallas, reglas de reservas, textos del sitio |
| **Datos** | **MongoDB Atlas** | Relojes, clientes, reservas, consultas, historial de stock |
| **Imágenes subidas desde el Admin** | **MongoDB Atlas** (dentro de la misma base) | Las fotos de los relojes |
| **Secretos y configuración** | **Render → Environment** | Contraseña de la base, clave de sesiones, email del Admin |
| **Imágenes fijas del diseño** | GitHub (carpeta `frontend/public`) | Fotos de muestra incluidas en el proyecto |

Consecuencia práctica: **subir código nuevo a GitHub nunca borra ni cambia
relojes, clientes ni reservas.** Y cargar un reloj en el Admin no toca el código.

Por eso el catálogo arrancó vacío: los relojes que veías en tu computadora
estaban en la base de datos de tu computadora, no en el código. La base de
Atlas es otra, nueva.

---

## 2. Qué sobrevive a cada situación

| Situación | Relojes | Clientes | Reservas | Consultas | Imágenes | Sesiones abiertas |
|---|---|---|---|---|---|---|
| Render reinicia el servidor | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Render vuelve a publicar (redeploy) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Subís código nuevo a GitHub | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| El servidor gratuito se duerme y se despierta | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Atlas reinicia o hace mantenimiento | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ (unos segundos sin servicio) |
| Cambiás `JWT_SECRET` en Render | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ todos deben volver a entrar |
| Se borra el cluster o la base en Atlas | ❌ | ❌ | ❌ | ❌ | ❌ | — |

Todo lo importante está en Atlas. El servidor de Render no guarda nada propio:
se puede reiniciar o reemplazar sin perder datos. **Lo único que puede hacer
perder datos es borrar cosas en Atlas**, y por eso los backups (sección 7).

Lo que sí se pierde en un reinicio, y no importa: el contador de intentos de
login (protección contra fuerza bruta), que vuelve a cero.

---

## 3. Cómo se actualiza el sitio

```
Cambiás código en tu computadora
        ↓  git add -A
        ↓  git commit -m "descripción del cambio"
        ↓  git push origin main
GitHub recibe el cambio
        ↓  (automático)
Render detecta el cambio, compila y publica
        ↓
Sitio actualizado (2 a 5 minutos)
```

- **El deploy automático está activado** (`autoDeploy: true` en `render.yaml`).
  No hace falta tocar nada en Render después de un `git push`.
- Render publica solo la parte que cambió: si tocaste `frontend/`, el frontend;
  si tocaste `backend/`, el backend.
- Si la versión nueva falla al compilar o no responde, **Render deja en línea la
  versión anterior**. El sitio no se cae por un push con errores.
- Cargar relojes, cambiar precios o atender reservas **no requiere** nada de
  esto: se hace desde el Admin.

---

## 4. Render: dónde está cada cosa

Entrá a https://dashboard.render.com . Hay dos servicios:

| Servicio | Qué es |
|---|---|
| `relojeria-yampier` | El sitio (frontend). |
| `relojeria-yampier-api` | El servidor (backend). Casi todo lo que vas a revisar está acá. |

Dentro de un servicio, menú de la izquierda:

| Sección | Para qué | Cuándo usarla |
|---|---|---|
| **Events** / **Deploys** | Historial de publicaciones, con su estado (Live, Failed). | Después de un `git push`, para ver si terminó bien. |
| **Logs** | Lo que va escribiendo el servidor, en vivo. | Cuando algo falla: buscá líneas `level=ERROR`. |
| **Environment** | Variables de configuración y secretos. | Para cambiar un dato de configuración. |
| **Metrics** | Uso de memoria y CPU. | Rara vez. |
| **Settings** | Plan (Free/Starter), dominio propio, health check. | Para pasar a plan pago o conectar un dominio. |

Botón **Manual Deploy** (arriba a la derecha del servicio):

| Opción | Qué hace | Cuándo |
|---|---|---|
| **Deploy latest commit** | Vuelve a compilar y publicar el último código. | Después de cambiar una variable `VITE_*` del frontend; o si un deploy falló por algo externo (como pasó con el acceso de red de Atlas). |
| **Restart service** | Reinicia el servidor sin recompilar. | Si el servidor quedó colgado; o después de cambiar algo en Atlas que requiera reconectar. |
| **Clear build cache & deploy** | Recompila desde cero. | Solo si un build falla sin motivo aparente. |

**Health (estado):** Render consulta `/api/health` para saber si el servidor
está vivo. Vos también podés abrir
https://relojeria-yampier-api.onrender.com/api/health :
- `{"database":"ok","status":"ok"}` → todo bien.
- No carga o tarda ~1 minuto → el servidor gratuito se está despertando.
- Error → mirá **Logs**.

Cambiar una variable en **Environment** y tocar **Save Changes** reinicia el
servicio solo.

---

## 5. Variables de entorno

Están en Render → servicio → **Environment**. Nunca se escriben en el código
ni se suben a GitHub.

### Backend (`relojeria-yampier-api`)

| Variable | Para qué sirve | ¿Secreta? | Ejemplo |
|---|---|---|---|
| `MONGO_URI` | Dirección y contraseña de la base en Atlas. | **Sí** | `mongodb+srv://usuario:****@yampier.xxxxx.mongodb.net/?appName=yampier` |
| `MONGO_DB` | Nombre de la base dentro de Atlas. | No | `relojeria_yampier` |
| `JWT_SECRET` | Clave con la que se firman las sesiones (Admin y clientes). Obligatoria. | **Sí** | (texto largo aleatorio generado por Render) |
| `ADMIN_EMAIL` | Email del Admin que se **crea** al arrancar, si no existe. | No (pero no publicarlo) | `admin@ejemplo.com` |
| `ADMIN_PASSWORD` | Contraseña con la que se **crea** ese Admin. | **Sí** | (contraseña fuerte) |
| `ALLOWED_ORIGIN` | Dirección(es) del sitio que pueden usar el servidor. Varias, separadas por coma. | No | `https://relojeria-yampier.onrender.com` |
| `APP_PUBLIC_URL` | Dirección del sitio, para armar links en emails. | No | `https://relojeria-yampier.onrender.com` |
| `APP_ENV` | `production` desactiva datos demo y funciones de prueba. | No | `production` |
| `SEED_DEMO_DATA` | Carga relojes de demostración. En producción se ignora aunque diga `true`. | No | `false` |
| `TRUSTED_PROXIES` | Permite conocer la IP real del visitante detrás de Render. | No | `10.0.0.0/8` |
| `RESERVATION_SWEEP_INTERVAL` | Cada cuánto se revisan reservas vencidas. | No | `5m` |
| `PORT` | Puerto del servidor. Lo define Render; no cargarlo. | No | — |
| `MAIL_DRIVER` | Envío de emails: `smtp` o vacío (no envía). | No | *(vacío hoy)* |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `MAIL_FROM` | Datos del proveedor de correo (solo si `MAIL_DRIVER=smtp`). | No | `smtp.proveedor.com`, `587` |
| `SMTP_PASSWORD` | Contraseña del correo. | **Sí** | — |
| `UPLOADS_DIR` | Carpeta de imágenes de versiones viejas (solo para migrarlas). No se usa en Render. | No | — |

### Frontend (`relojeria-yampier`)

Estas **no son secretas**: quedan a la vista dentro del sitio publicado.
Después de cambiarlas hay que hacer **Manual Deploy → Deploy latest commit**.

| Variable | Para qué sirve | Ejemplo |
|---|---|---|
| `VITE_API_URL` | Dirección del servidor. Debe ser `https` y terminar en `/api`. | `https://relojeria-yampier-api.onrender.com/api` |
| `VITE_WHATSAPP_NUMBER` | WhatsApp del local, solo números con código de país. Vacío = no se muestra el botón. | `549351XXXXXXX` |
| `VITE_CONTACT_EMAIL` | Email público del local. Vacío = no se muestra. | `contacto@ejemplo.com` |
| `VITE_INSTAGRAM_URL`, `VITE_FACEBOOK_URL` | Links a redes. Vacío = no se muestran. | `https://instagram.com/...` |
| `NODE_VERSION` | Versión de Node para compilar. No tocar. | `20` |

---

## 6. MongoDB Atlas: cómo mirar los datos

Sirve para **comprobar**, no para administrar. El trabajo diario se hace desde
el panel Admin del sitio.

1. Entrá a https://cloud.mongodb.com
2. Menú izquierdo → **Clusters** → en `yampier`, botón **Browse Collections**
   (o menú **Data Explorer**).
3. A la izquierda, abrí la base **`relojeria_yampier`**.

Aparece recién cuando el servidor arrancó por primera vez. Colecciones:

| Colección | Qué guarda | Qué deberías ver |
|---|---|---|
| `watches` | Los relojes. | Un documento por reloj cargado. Vacía hasta que cargues el primero. |
| `admins` | Usuarios del panel Admin. | 1 documento, con tu `ADMIN_EMAIL`. La contraseña está cifrada (`passwordHash`), no se puede leer. |
| `customers` | Cuentas de clientes. | Un documento por cliente registrado. Contraseñas cifradas. |
| `reservations` | Reservas. | Una por solicitud, con su estado (`pending`, `confirmed`, `deposit_paid`, `completed`, `cancelled`, `expired`). |
| `inquiries` | Consultas del formulario de contacto. | Una por mensaje (`new`, `contacted`, `closed`). |
| `stock_movements` | Historial de stock. | Una línea por cada cambio de stock. |
| `uploads.files` | Lista de imágenes subidas. | Una por foto (nombre, tamaño, fecha). |
| `uploads.chunks` | El contenido de esas imágenes, en partes. | Varias por foto. No se "ven" como imagen acá. |
| `password_resets` | Pedidos de recuperación de contraseña. | Normalmente vacía. |
| `app_meta` | Marcas internas del sistema. | Puede no existir en producción. |

Algunas colecciones aparecen recién cuando se guarda el primer dato (por
ejemplo, `uploads.files` aparece al subir la primera foto).

**Otras bases que puedas ver** (`sample_mflix`, `sample_...`): son datos de
ejemplo que Atlas ofrece cargar al crear el cluster. El sitio no las usa. Ocupan
parte de los 512 MB gratuitos; si aparecen, se pueden borrar desde el cluster
(menú **…** → *Delete Sample Dataset* o tacho junto a cada base `sample_`).
**No borres `relojeria_yampier`.**

**No edites documentos a mano** salvo para un diagnóstico puntual indicado en
esta guía. Cambiar un stock o un estado a mano saltea las reglas del sistema
(historial, unidades apartadas) y deja los datos inconsistentes.

---

## 7. Backups

El plan gratuito de Atlas (M0) **no hace copias automáticas**. Un solo backup
cubre todo, porque datos e imágenes están en la misma base.

Con Docker Desktop abierto, en PowerShell (reemplazá `TU_MONGO_URI` por la
dirección completa, la misma de Render):

```bash
docker run --rm -v "${PWD}:/backup" mongo:7 mongodump --uri "TU_MONGO_URI" --db relojeria_yampier --gzip --archive=/backup/yampier-backup.archive.gz
```

- Hacelo **una vez por semana** y antes de cualquier cambio grande.
- Renombrá el archivo con la fecha y guardalo **fuera** de la carpeta del
  proyecto (contiene datos personales de clientes).
- Cuando el negocio dependa del sistema, conviene un plan pago de Atlas con
  copias automáticas.

---

## 8. Problemas frecuentes

| Síntoma | Causa probable | Qué hacer |
|---|---|---|
| El sitio abre pero el catálogo tarda ~1 minuto | El servidor gratuito estaba dormido. | Esperar. Para evitarlo: plan Starter en Render (Settings → Instance Type). |
| El catálogo está vacío | No hay relojes cargados, o están archivados, o tienen una marca distinta de "Casio". | Admin → Relojes. Revisar filtro **Estado** y el campo **Marca**. |
| "No pudimos cargar los relojes" | El servidor no responde. | Abrir `/api/health`. Si falla, Render → backend → Logs. |
| Deploy **Failed** con "MongoDB no responde" | Atlas no acepta la conexión. | Atlas → Network Access → debe estar `0.0.0.0/0` **Active**. Luego Manual Deploy. |
| Deploy **Failed** con "bad auth" / "Authentication failed" | Usuario o contraseña mal en `MONGO_URI`. | Render → Environment → corregir `MONGO_URI`. |
| El build del frontend falla con "VITE_API_URL inválida" | Falta la variable o no es `https://…/api`. | Render → frontend → Environment → corregir → Manual Deploy. |
| Una foto no se ve | La subida no terminó antes de guardar. | Editar el reloj, volver a elegir la imagen, esperar la vista previa y Guardar. |
| No puedo entrar al Admin | Contraseña incorrecta o bloqueo por intentos (1 minuto). | Esperar 1 minuto. Si la olvidaste, ver abajo. |

### Olvidé la contraseña del Admin

Dato importante: **cambiar `ADMIN_PASSWORD` en Render no cambia la contraseña de
un Admin que ya existe.** Esa variable solo se usa para *crear* el Admin cuando
todavía no hay ninguno con ese email. Si ya existe, el sistema no lo toca.

Para volver a entrar:

1. Render → `relojeria-yampier-api` → **Environment** → cambiá `ADMIN_PASSWORD`
   por la contraseña nueva. Todavía no guardes si podés; si guardás, no pasa nada.
2. Atlas → **Browse Collections** → `relojeria_yampier` → `admins`.
3. Borrá **solo** el documento con tu email (ícono de tacho en ese documento).
   Es la única edición manual que indica esta guía.
4. Render → **Save Changes** (o **Manual Deploy → Restart service**).
5. Al arrancar, el sistema no encuentra un Admin con ese email y lo crea de
   nuevo con la contraseña nueva. En **Logs** vas a ver
   `usuario admin inicial creado`.
6. Entrá al panel con la contraseña nueva.

Los relojes, reservas y clientes no se ven afectados.

---

## 9. Seguridad: lo que está cubierto y lo que depende de vos

Ya cubierto por el sistema:

- Ningún secreto está en GitHub (verificado, incluido el historial).
- Todo el tráfico es HTTPS.
- El servidor solo acepta pedidos desde el sitio propio (CORS).
- El panel Admin exige sesión; las contraseñas se guardan cifradas.
- Límite de intentos en logins, registro y formulario de contacto.
- Las imágenes se validan por contenido real (no por el nombre del archivo).
- Los errores internos no se muestran al visitante.
- Los relojes de demostración no se cargan en producción.

Depende de vos:

- **No compartir** `MONGO_URI`, `ADMIN_PASSWORD` ni `JWT_SECRET` por chat, mail
  o capturas de pantalla. Si alguno se compartió, cambiarlo ("rotarlo").
- Cómo rotar la contraseña de la base: Atlas → *Database & Network Access* →
  *Database Users* → **Edit** → *Edit Password* → *Autogenerate* → **Update
  User**. Después, Render → backend → Environment → actualizar `MONGO_URI` con
  la contraseña nueva → **Save Changes**.
- Activar la verificación en dos pasos en GitHub, Render y Atlas.
- Hacer backups (sección 7).
- Cerrar sesión del Admin en computadoras compartidas.

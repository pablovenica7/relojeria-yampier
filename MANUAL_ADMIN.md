# Manual del Admin — Relojería Yampier

Para quien atiende el local y maneja el sitio. No hace falta saber programar.

- Sitio público: https://relojeria-yampier.onrender.com/
- Panel Admin: https://relojeria-yampier.onrender.com/admin

> Si nadie entró al sitio en los últimos 15 minutos, la primera pantalla puede
> tardar hasta 1 minuto en cargar los datos (el servidor gratuito "se
> despierta"). Es normal: esperá, no hace falta recargar muchas veces.

---

## 1. Entrar al panel

1. Abrí https://relojeria-yampier.onrender.com/admin
2. Si no tenés la sesión abierta, te lleva a la pantalla **Panel Admin**.
3. Escribí el **Email** y la **Contraseña** y tocá **Ingresar**.

**¿De dónde salen el email y la contraseña?** Son los valores `ADMIN_EMAIL` y
`ADMIN_PASSWORD` que se cargaron en Render al publicar el sitio. Con ellos el
sistema creó el usuario Admin la primera vez que arrancó.

**La sesión dura 24 horas.** Después te vuelve a pedir la contraseña.
Para salir antes: botón **Cerrar sesión** (abajo a la izquierda). Hacelo
siempre en computadoras compartidas.

**Si escribís mal la contraseña** varias veces seguidas, el sistema bloquea los
intentos por 1 minuto. Esperá y probá de nuevo.

**Si olvidaste la contraseña:** el panel no tiene "recuperar contraseña" ni
"cambiar contraseña" para el Admin. Cambiar `ADMIN_PASSWORD` en Render **no**
cambia la contraseña de un Admin que ya existe. La solución está en
[GUIA_PRODUCCION.md](GUIA_PRODUCCION.md#olvidé-la-contraseña-del-admin).

---

## 2. Las secciones del panel

Menú de la izquierda:

| Sección | Para qué sirve |
|---|---|
| **Inicio** | Resumen: reservas pendientes, confirmadas, con seña, consultas nuevas, relojes sin stock y con última unidad. Cada número es un link a la lista ya filtrada. También muestra las reservas que vencen en 48 h. |
| **Relojes** | El catálogo: crear, editar, stock, archivar. |
| **Reservas** | Solicitudes de clientes y reservas cargadas por el local. |
| **Stock** | Historial de todos los movimientos de stock (quién, cuándo, por qué). |
| **Clientes** | Cuentas creadas en el sitio. Se pueden desactivar y reactivar. |
| **Consultas** | Mensajes enviados desde el formulario de Contacto. |

---

## 3. Relojes

### Crear un reloj

1. **Relojes** → botón **Nuevo reloj**.
2. Completá el formulario:

| Campo | ¿Obligatorio? | Qué poner |
|---|---|---|
| **Marca** | Sí | Viene con `Casio`. **Dejala en "Casio"**: el catálogo público hoy muestra solo relojes de marca Casio. Un reloj con otra marca se guarda pero **no aparece** en el sitio. |
| **Nombre** | Sí | Como se va a ver en el catálogo. Ej: `Casio G-Shock GA-2100-1A1`. Máximo 120 caracteres. |
| **Modelo** | Sí | El código comercial. Ej: `GA-2100-1A1`. Solo letras, números, espacios y `- . / _`. **No se puede repetir**: dos relojes no pueden tener el mismo modelo. Se guarda en mayúsculas. |
| **Género** | Sí | `Hombre` o `Mujer`. Define en qué categoría del menú aparece. |
| **Unidades en stock** | Sí | Cantidad real que hay en el local (0 o más). Debajo ves cómo se va a mostrar ("En stock", "Última unidad", "Sin stock"). |
| **Precio (ARS)** | No | Número entero en pesos, sin centavos ni puntos. Ej: `185000`. Vacío = el sitio muestra "Consultar precio". |
| **Descripción** | No | Texto libre. |
| **Especificaciones** | No | **Una por línea.** Cada línea sale como un ítem de la lista. |
| **Imagen** | No | Foto JPG, PNG o WEBP de hasta 5 MB. Sin foto, el sitio muestra un dibujo de reloj. |

3. Para la foto: tocá **Imagen**, elegí el archivo y **esperá** a que termine
   "Subiendo imagen…" y aparezca la vista previa.
4. Tocá **Guardar**. Aparece "Reloj creado" y volvés a la lista.

El reloj aparece en el sitio **inmediatamente**. No hay que publicar ni
reiniciar nada. Si no lo ves, recargá la página del catálogo.

### Ejemplo de carga (datos ficticios, solo para practicar)

```
Marca:              Casio
Nombre:             Casio Vintage Ejemplo A-100
Modelo:             A-100-EJEMPLO
Género:             Hombre
Unidades en stock:  2
Precio (ARS):       85000
Descripción:        Reloj digital clásico con caja cromada y malla de acero.
Especificaciones:   Cronómetro
                    Alarma diaria
                    Luz LED
                    Resistente a salpicaduras
Imagen:             (una foto .jpg del reloj)
```

Cuando termines de practicar, **archivá** ese reloj de ejemplo (ver abajo).

### Editar un reloj (precio, descripción, especificaciones, foto)

1. **Relojes** → buscá el reloj (podés usar **Buscar**, por nombre, marca o modelo).
2. Tocá **Editar**.
3. Cambiá lo que necesites:
   - **Precio**: escribí el nuevo número. Las reservas que ya existen
     **conservan el precio que tenían** cuando se hicieron.
   - **Descripción / Especificaciones**: editá el texto.
   - **Cambiar la foto**: tocá **Imagen** y elegí otro archivo. Esperá la
     vista previa nueva.
4. **Guardar**.

### Cambiar el stock

Hay dos formas:

**A) Rápida, desde la lista:** botones **−** y **+** al lado de la cantidad.
Cada toque suma o resta 1 unidad.

**B) Desde Editar:** cambiá **Unidades en stock**. Al cambiar el número aparecen
dos campos más:
- **Tipo de movimiento**: *Ingreso de mercadería* (llegaron relojes),
  *Ajuste manual* o *Corrección de inventario* (había un error de conteo).
- **Motivo**: texto libre que queda en el historial. Ej: "Llegó pedido de octubre".

Todo cambio queda registrado en **Stock** (o en el link **Historial** del reloj).

> **No bajes el stock a mano cuando confirmás una reserva.** El sistema ya
> descuenta 1 unidad solo. Si además lo bajás a mano, queda descontado dos veces.

Si al guardar aparece *"El stock cambió mientras editabas"*: alguien confirmó
una reserva o tocó el stock mientras tenías el formulario abierto. Recargá la
página y volvé a hacer el cambio.

### Qué ve el cliente según el stock

| Stock | Etiqueta | En el catálogo | En la ficha del reloj |
|---|---|---|---|
| **0** | **Sin stock** | Aparece, con la etiqueta "Sin stock". No aparece en la "Selección Casio" de la página de inicio. | **No** hay botón Reservar. Hay "Consultar reposición" y un aviso de que no está disponible. |
| **1** | **Última unidad** | Aparece con la etiqueta "Última unidad". | Botón **Reservar** y "Consultar disponibilidad". |
| **2 o más** | **En stock** | Aparece con la etiqueta "En stock". | Botón **Reservar** y "Consultar disponibilidad". |

El cliente nunca ve la cantidad exacta, solo la etiqueta.

### Archivar y restaurar

**Archivar** = sacar el reloj del sitio sin borrarlo.

1. **Relojes** → en la fila del reloj, **Archivar** → confirmar.
2. Deja de verse en el sitio. Sus reservas y consultas se conservan.
3. Si tiene reservas activas, el sistema avisa cuántas y pide una segunda
   confirmación ("Archivar igualmente"). Esas reservas siguen en **Reservas**.

**Restaurar:**

1. **Relojes** → filtro **Estado** → **Archivados**.
2. En la fila del reloj, **Restaurar**. Vuelve al sitio en el momento.

**Eliminar definitivamente** solo aparece en relojes archivados y solo funciona
si el reloj **nunca tuvo reservas**. No se puede deshacer. Para el día a día,
usá Archivar.

---

## 4. Reservas

### Regla más importante

> **Una solicitud "Pendiente" NO aparta el reloj.** El reloj recién queda
> apartado (y se descuenta 1 unidad del stock) cuando el local toca
> **Confirmar**.

### Los estados y qué hacer en cada uno

| Estado (panel) | Qué significa | ¿Retiene 1 unidad? | Qué hace el empleado |
|---|---|---|---|
| **Pendiente** | El cliente pidió la reserva desde la web (o el local la cargó como pendiente). | No | Revisar que el reloj esté físicamente en el local, contactar al cliente y tocar **Confirmar (retiene 1 unidad)** o **Cancelar**. |
| **Confirmada** | El local aceptó. El reloj está apartado. | Sí | Coordinar la seña o el retiro. Botones: **Registrar seña**, **Completar venta**, **Cancelar**. |
| **Seña paga** | El cliente dejó una seña. | Sí | Esperar a que retire. Botones: **Completar venta**, **Cancelar**. |
| **Completada** | El cliente pagó y se llevó el reloj. | No (la unidad ya se vendió) | Nada. Estado final. |
| **Cancelada** | Se canceló (el cliente o el local). | No (si estaba apartada, la unidad vuelve al stock) | Si hubo seña, devolverla en el local según lo acordado. Estado final. |
| **Vencida** | Pasó la fecha de vencimiento sin completarse. Lo marca el sistema solo. | No (la unidad vuelve al stock) | Nada. Estado final. |

Los estados finales no se pueden reabrir. Si hace falta, se crea una reserva nueva.

### Atender una solicitud web (paso a paso)

1. **Inicio** → tarjeta **Reservas pendientes** (o **Reservas** → Estado: Pendiente).
2. Abrí la tarjeta de la reserva. Vas a ver reloj, cliente, teléfono, email,
   precio reservado y el comentario del cliente. En **Datos del cliente (cuenta
   web)** están documento y datos fiscales.
3. Verificá que el reloj esté en el local.
4. Contactá al cliente (el teléfono y el email son links).
5. Tocá **Confirmar (retiene 1 unidad)** → **Sí, continuar**.
   El stock del reloj baja 1 automáticamente.
6. Si el cliente deja seña: **Registrar seña** → escribí el **Monto recibido**
   y elegí el **Método** (Efectivo, Transferencia bancaria, Tarjeta en el
   local, Otro) → **Registrar seña**. Es solo un registro: el sitio no cobra nada.
7. Cuando el cliente paga el resto y se lleva el reloj: **Completar venta**.

**Para completar hay que confirmar antes.** De "Pendiente" no se puede pasar
directo a "Completada".

### Vencimientos

- Una solicitud web **pendiente** vence a los **7 días**.
- Una reserva cargada por el local vence a los **3 días** por defecto (se
  puede cambiar al crearla).
- **Pendientes y confirmadas vencen solas.** Al vencer, si tenían una unidad
  apartada, vuelve al stock.
- Una reserva **con seña paga no vence sola**: la decide el local.
- Para darle más tiempo a un cliente: en la tarjeta, cambiá **Vencimiento** y
  tocá **Guardar cambios**.

### Notas

El campo **Notas** de cada reserva es interno (el cliente no lo ve). Escribí y
tocá **Guardar cambios**.

### Cargar una reserva desde el local (cliente por teléfono o en persona)

1. **Reservas** → **Nueva reserva**.
2. Elegí el **Reloj**, completá **Cliente** y al menos **Teléfono** o **Email**.
3. **Estado inicial**: *Pendiente (no retiene stock)* o *Confirmada (retiene 1 unidad)*.
4. Opcionales: **Precio acordado**, **Seña acordada**, **Vence**, **Notas**.
5. **Crear reserva**.

### Mensajes que pueden aparecer

| Mensaje | Qué pasó | Qué hacer |
|---|---|---|
| "No hay stock disponible para este reloj" | Quisiste confirmar y ya no quedan unidades (por ejemplo, se confirmó otra reserva antes). | Dejarla pendiente hasta reponer, o cancelarla y avisar al cliente. |
| "La reserva fue modificada al mismo tiempo. Recargá y volvé a intentar." | Otra persona (o el cliente) cambió la reserva en el mismo momento. | Recargar la página y mirar el estado actual. |
| "No se puede pasar una reserva de … a …" | Ese cambio de estado no está permitido. | Revisar la tabla de estados. |
| "La seña es inválida o supera el precio" | El monto es 0, negativo o mayor al precio reservado. | Corregir el monto. |

### Dos clientes quieren la última unidad

Con stock = 1, los dos pueden **solicitar** la reserva: las dos quedan
pendientes y ninguna aparta el reloj. Se lo queda **el primero que el local
confirme**. Al confirmar la primera, el stock pasa a 0. Si intentás confirmar la
segunda, el sistema lo impide con *"No hay stock disponible para este reloj"*.
Cancelá esa segunda y avisale al cliente. El sistema garantiza que una misma
unidad nunca se aparta dos veces, aunque dos empleados toquen "Confirmar" al
mismo tiempo.

---

## 5. Consultas

Son los mensajes del formulario **Contacto** del sitio. **No descuentan stock.**

| Estado | Significa |
|---|---|
| **Nueva** | Nadie la respondió todavía. |
| **Contactado** | Ya se le respondió al cliente. |
| **Cerrada** | Tema terminado. |

Cómo trabajarlas:

1. **Inicio** → **Consultas nuevas** (o **Consultas** → Estado: Nueva).
2. Leé el mensaje. Si el cliente consultó desde la ficha de un reloj, se
   indica cuál ("Reloj consultado").
3. Respondé por teléfono, WhatsApp o email (el sitio no envía la respuesta).
4. Cambiá el **Estado** a **Contactado**. Se guarda solo.
5. Opcional: escribí **Notas internas** y tocá **Guardar notas**.
6. Si el cliente quiere el reloj: link **Crear reserva** (abre el formulario con
   sus datos ya cargados).
7. Cuando termine el tema: Estado **Cerrada**.

**Eliminar** es solo para spam o errores. Para consultas atendidas, usá Cerrada.

---

## 6. Clientes

Lista de cuentas creadas en el sitio, con buscador por nombre, email o documento.

- **Desactivar**: el cliente no puede volver a entrar y se le cierran las
  sesiones abiertas. Sus reservas se conservan.
- **Reactivar**: puede volver a entrar con su contraseña de siempre.

Las cuentas no se borran, porque tienen historial de reservas.

**Si un cliente olvidó su contraseña:** hoy el sitio no envía el email de
recuperación (falta configurar el correo, ver GUIA_PRODUCCION.md). Hasta
entonces no hay forma de restablecerla desde el panel.

---

## 7. Rutina diaria sugerida

1. Entrar al panel y mirar **Inicio**.
2. **Reservas pendientes** → confirmar o cancelar cada una.
3. **Consultas nuevas** → responder y marcar como Contactado.
4. **Reservas que vencen en las próximas 48 h** → llamar al cliente o extender.
5. Si llegó mercadería: **Relojes** → sumar stock (tipo *Ingreso de mercadería*).
6. **Cerrar sesión** al terminar.

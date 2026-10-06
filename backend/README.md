# Backend — arquitectura

Monolito modular en Go (`net/http` estándar) + MongoDB. Un solo binario,
dividido internamente por responsabilidad. Inspirado en la separación
`controller / service / repository / domain / dto / app` del proyecto de
referencia `reservas-hoteles`, pero **sin** microservicios, mensajería ni
caches: Yampier no los necesita.

```
HTTP request
   ↓
middleware   CORS · request ID + log · recovery · timeout · rate limit · auth (admin / cliente)
   ↓
controller   lee el request, valida FORMATO, llama al service, escribe la respuesta
   ↓
service      TODAS las reglas de negocio (stock, reservas, cuentas, consultas)
   ↓
repository   solo persistencia MongoDB (operaciones atómicas incluidas)
   ↓
MongoDB
```

`dto` (payloads HTTP) ↔ `service` ↔ `domain` (entidades y reglas puras).

## Carpetas

```
backend/
├── cmd/api/main.go        arranque: config → Mongo → app → Run (graceful shutdown)
└── internal/
    ├── app/               composición explícita de dependencias y rutas por dominio
    ├── config/            única lectura de variables de entorno (valida y falla rápido)
    ├── database/          conexión, nombres de colecciones, índices, backfill y seeds
    ├── domain/            entidades, estados, transiciones, validaciones puras, errores de dominio
    ├── dto/               requests y responses HTTP (contrato con el frontend)
    ├── repository/        persistencia MongoDB (una struct por colección)
    ├── service/           reglas de negocio (Stock, Reservation, Watch, Customer, ...)
    ├── controller/        handlers HTTP
    ├── middleware/        CORS, logs, recovery, timeout, rate limit, auth
    ├── auth/              JWT (claims, roles, tokenVersion) y contraseñas (bcrypt)
    ├── httpx/             WriteJSON / WriteError / DecodeJSON y mapeo error → status
    └── mail/              envío de emails (smtp | log | deshabilitado)
```

## Responsabilidad de cada capa

| Capa | Hace | No hace |
|---|---|---|
| controller | decodificar JSON, ids y query params; usuario autenticado del contexto; llamar al service; responder | consultas Mongo, reglas de stock/reservas, hashing |
| service | validar reglas de negocio, transiciones de estado, stock, snapshots, ledger | conocer HTTP o status codes |
| repository | traducir entidades ↔ documentos; filtros y operaciones atómicas de Mongo | decidir reglas de negocio |
| domain | entidades, constantes, transiciones válidas, validaciones puras | depender de HTTP o del driver de conexión |

## Reglas críticas (dónde viven)

- **Reservas**: `service/reservation.go` es el único lugar donde cambia un
  estado (`Confirm`, `MarkDepositPaid`, `Complete`, `Cancel`,
  `CancelByCustomer`, `ExpireDue`). Las transiciones válidas están en
  `domain/reservation.go`.
- **Stock**: `service/stock.go` (`HoldUnit`, `ReleaseUnit`, `Adjust`,
  `SetQuantity`) es el único que modifica `stockQuantity` y anota el libro de
  movimientos. Las operaciones son atómicas en `repository/watch.go`
  (condición + cambio en un solo `FindOneAndUpdate`, nunca "leer, restar en
  Go, guardar").

## Errores

Los services devuelven `*domain.Error` con un tipo (`ErrValidation`,
`ErrNotFound`, `ErrConflict`, `ErrUnauthorized`, `ErrForbidden`,
`ErrOutOfStock`, `ErrInvalidTransition`, `ErrConcurrentUpdate`). `httpx.WriteError`
los traduce a 400/401/403/404/409 en un solo lugar. Cualquier otro error es
un 500 genérico: el detalle interno va al log, nunca al cliente.

## Dependencias e interfaces

Sin globals ni `init()`: `app.New` arma `repositories → services →
controllers → router`. Cada service declara interfaces chicas con lo que usa
de los repositories (definidas por el consumidor), lo que permite testear las
reglas con fakes en memoria.

## Transacciones

MongoDB corre como instancia única (sin replica set), así que no hay
transacciones multi-documento. **Decisión: no activarlas ahora.** El riesgo de
cambiar la infraestructura (replica set en Docker, backups, operación) supera
el beneficio con el diseño actual:

- cada cambio de stock es una operación atómica condicional (la última unidad
  no se puede reservar dos veces);
- los cambios de estado de reservas son condicionales al estado leído;
- el orden de los pasos hace que un corte a mitad de operación solo pueda dejar
  el stock **por debajo** del real (corregible desde el Admin), nunca reservar
  de más; las compensaciones devuelven lo retenido si el segundo paso falla.

Si en el futuro se agrega un replica set, el lugar natural para envolver en
una transacción es `ReservationService.apply`.

## Tests

```bash
go test ./...
```

- **Unitarios** (sin Mongo ni Docker): `domain`, `dto`, `config`, `auth`,
  `middleware` y `service`. Los de `service` usan fakes en memoria y cubren
  confirmación, última unidad concurrente, cancelación, vencimiento, venta,
  stock nunca negativo, ownership, tokenVersion y reset de contraseña.
- **Integración** (`internal/app`): router HTTP real + MongoDB real en una base
  temporal `yampier_test_<id>` que se borra al terminar. Usan `MONGO_TEST_URI`
  o `localhost:27017`; si no hay Mongo, se saltean.
